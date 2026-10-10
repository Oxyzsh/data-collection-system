package http

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	alarmdomain "data-collection-system/internal/templatealarm"
	"data-collection-system/internal/bootstrap"
	"data-collection-system/internal/interfaces/httpx/locale"
	"data-collection-system/internal/interfaces/httpx/response"
	apperrors "data-collection-system/internal/shared/errors"
	"data-collection-system/internal/shared/i18n"
	"data-collection-system/internal/shared/logger"
	templatemodule "data-collection-system/internal/template"
	templateapp "data-collection-system/internal/template/application"
	templatebiz "data-collection-system/internal/template/domain"

	"github.com/gin-gonic/gin"
)

const maxFileSize = 10 * 1024 * 1024

// 模板 handler 主要围绕三类场景：
// 1. 查模板详情；
// 2. 预检查/替换模板；
// 3. 上传文件并解析模板。
// 它本身不解析模板文件，也不直接操作模板缓存，只负责把 HTTP 请求转成 application 输入。

// ReplaceTemplateRequest 是替换模板时的请求体。
type ReplaceTemplateRequest struct {
	TemplateName string                       `json:"template_name"`
	Description  string                       `json:"description"`
	TemplateType string                       `json:"template_type"`
	Points       []templatemodule.DevicePoint `json:"points"`
	Alarms       []alarmdomain.DeviceAlarm    `json:"alarms"`
}

// ImportPreview 是模板导入预览结构。
type ImportPreview struct {
	TemplateName string                           `json:"template_name"`
	Description  string                           `json:"description"`
	TemplateType string                           `json:"template_type"`
	Points       []templatemodule.DevicePoint     `json:"points"`
	Alarms       []templatemodule.NormalizedAlarm `json:"alarms"`
}

// TemplateReviewResponse 是模板 review 接口的响应结构。
type TemplateReviewResponse struct {
	TemplateID       int                                `json:"template_id"`
	TemplateName     string                             `json:"template_name"`
	TemplateType     string                             `json:"template_type"`
	ReferenceCount   int                                `json:"reference_count"`
	ReferenceDevices []templatemodule.TemplateDeviceRef `json:"reference_devices"`
	PointCount       int                                `json:"point_count"`
	AlarmCount       int                                `json:"alarm_count"`
	CheckResult      string                             `json:"check_rslt"`
	Warnings         []string                           `json:"warnings"`
}

// toTemplateDeviceRefs 把 domain 层引用信息转换成 HTTP 返回结构。
func toTemplateDeviceRefs(refs []templatebiz.TemplateReference) []templatemodule.TemplateDeviceRef {
	out := make([]templatemodule.TemplateDeviceRef, 0, len(refs))
	for _, ref := range refs {
		enabled := 0
		if ref.Enabled {
			enabled = 1
		}
		out = append(out, templatemodule.TemplateDeviceRef{DeviceID: ref.DeviceID, Name: ref.Name, Status: ref.Status, Enabled: enabled})
	}
	return out
}

// GetDeviceTemplates 返回模板列表。
// 列表来自模板缓存，是最轻量的一类模板查询。
func GetDeviceTemplates(c *gin.Context) {
	service := templatemodule.GetQueryService()
	response.API(c, 200, 200, service.GetTemplates(), i18n.KeyCommonSuccess)
}

// GetTemplateDetail 返回模板详情，包含语言本地化后的测点和告警。
// 这里会把 domain 里的原始点位/告警转换成适合当前语言展示的归一化视图。
func GetTemplateDetail(c *gin.Context) {
	id := c.Param("id")
	isEnglish := locale.IsEnglish(c)
	templateID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, i18n.KeyTemplateInvalidID)
		return
	}

	service := templatemodule.GetQueryService()
	detail, err := service.GetTemplateDetail(templateID)
	if err != nil {
		if errors.Is(err, templateapp.ErrTemplateNotFound) {
			response.API(c, 404, 404, nil, i18n.KeyTemplateNotFound)
			return
		}
		response.API(c, 500, 500, nil, "查询模板失败")
		return
	}

	typ := detail.Template.Type
	response.API(c, 200, 200, templatemodule.TemplateDetailResponse{
		Template:   *detail.Template,
		Points:     templatemodule.NormalizePointsWithLang(typ, detail.Points, isEnglish),
		Alarms:     templatemodule.NormalizeAlarmsWithLang(typ, detail.Alarms, isEnglish),
		References: toTemplateDeviceRefs(detail.References),
	}, i18n.KeyCommonSuccess)
}

// GetDeviceTemplateByID 返回模板的原始详情。
// 和 detail 接口相比，这里更接近“编辑态原始数据”，方便前端回填表单。
func GetDeviceTemplateByID(c *gin.Context) {
	id := c.Param("id")
	isEnglish := locale.IsEnglish(c)
	templateID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, i18n.KeyTemplateInvalidID)
		return
	}

	service := templatemodule.GetQueryService()
	detail, err := service.GetTemplateDetail(templateID)
	if err != nil {
		if errors.Is(err, templateapp.ErrTemplateNotFound) {
			response.API(c, 404, 404, nil, i18n.KeyTemplateNotFound)
			return
		}
		response.API(c, 500, 500, nil, "查询模板失败")
		return
	}

	typ := detail.Template.Type
	response.API(c, 200, 200, gin.H{
		"template": detail.Template,
		"points":   templatemodule.NormalizePointsWithLang(typ, detail.Points, isEnglish),
		"alarms":   templatemodule.NormalizeAlarmsWithLang(typ, detail.Alarms, isEnglish),
	}, i18n.KeyCommonSuccess)
}

// ReviewTemplate 在不落库的情况下检查模板修改结果。
// 它相当于模板替换前的一次 dry-run，帮助前端先看到引用影响和潜在警告。
func ReviewTemplate(c *gin.Context) {
	id := c.Param("id")
	templateID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, i18n.KeyTemplateInvalidID)
		return
	}

	var req ReplaceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	service := templatemodule.GetQueryService()
	review, err := service.ReviewTemplate(templateapp.ReviewTemplateInput{
		TemplateID:   templateID,
		TemplateName: req.TemplateName,
		TemplateType: req.TemplateType,
		Points:       req.Points,
		Alarms:       req.Alarms,
	})
	if err != nil {
		if errors.Is(err, templateapp.ErrTemplateNotFound) {
			response.API(c, 404, 404, nil, i18n.KeyTemplateNotFound)
			return
		}
		if errors.Is(err, templateapp.ErrTemplateTypeImmutable) {
			response.APIWithArgs(c, 400, 400, nil, "template.type_immutable", map[string]interface{}{
				"type": templateImmutableType(err),
			})
			return
		}
		response.API(c, 500, 500, nil, "模板检查失败")
		return
	}

	response.API(c, 200, 200, TemplateReviewResponse{
		TemplateID:       review.TemplateID,
		TemplateName:     review.TemplateName,
		TemplateType:     review.TemplateType,
		ReferenceCount:   review.ReferenceCount,
		ReferenceDevices: toTemplateDeviceRefs(review.References),
		PointCount:       review.PointCount,
		AlarmCount:       review.AlarmCount,
		CheckResult:      joinTemplateWarnings(c, review.Warnings),
		Warnings:         localizeTemplateWarnings(c, review.Warnings),
	}, i18n.KeyCommonSuccess)
}

// DeleteTemplate 删除模板。
// 真正能否删除由 application 决定：必须满足“非内置、且无设备引用”。
func DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	templateID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, i18n.KeyTemplateInvalidID)
		return
	}

	service := templatemodule.GetConfigService()
	if err := service.DeleteTemplate(templateID); err != nil {
		if errors.Is(err, templateapp.ErrTemplateNotFound) {
			response.API(c, 404, 404, nil, i18n.KeyTemplateNotFound)
			return
		}
		if errors.Is(err, templateapp.ErrTemplateBuiltIn) {
			response.API(c, 400, 400, nil, "内置模板不允许删除")
			return
		}
		var refErr *templateapp.TemplateReferencedError
		if errors.As(err, &refErr) {
			deviceNames := make([]string, 0, len(refErr.References))
			for _, ref := range refErr.References {
				if strings.TrimSpace(ref.Name) != "" {
					deviceNames = append(deviceNames, ref.Name)
				}
			}
			msg := response.TextWithArgs(c, "template.delete.referenced", map[string]interface{}{
				"count": len(refErr.References),
			})
			if len(deviceNames) > 0 {
				devicesSeparator := "、"
				if locale.IsEnglish(c) {
					devicesSeparator = ", "
				}
				msg = response.TextWithArgs(c, "template.delete.referenced_with_devices", map[string]interface{}{
					"count":   len(refErr.References),
					"devices": strings.Join(deviceNames, devicesSeparator),
				})
			}
			response.API(c, 400, 400, nil, msg)
			return
		}
		response.API(c, 500, 500, nil, "删除模板失败")
		return
	}

	response.API(c, 200, 200, gin.H{"id": templateID}, i18n.KeyCommonDeleteSuccess)
}

// ReplaceTemplate 替换模板并触发重建。
// 这里的“成功”不只意味着模板保存成功，还可能包含“关联运行设备是否重建成功”这一层结果。
func ReplaceTemplate(c *gin.Context) {
	id := c.Param("id")
	templateID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, i18n.KeyTemplateInvalidID)
		return
	}

	var req ReplaceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	service := templatemodule.GetConfigService()
	result, err := service.ReplaceTemplate(templateapp.ReplaceTemplateInput{
		TemplateID:   templateID,
		TemplateName: req.TemplateName,
		Description:  req.Description,
		TemplateType: req.TemplateType,
		Points:       req.Points,
		Alarms:       req.Alarms,
	})
	if err != nil {
		if errors.Is(err, templateapp.ErrTemplateNotFound) {
			response.API(c, 404, 404, nil, i18n.KeyTemplateNotFound)
			return
		}
		if errors.Is(err, templateapp.ErrTemplateBuiltIn) {
			response.API(c, 400, 400, nil, "内置模板不允许替换")
			return
		}
		if errors.Is(err, templateapp.ErrTemplateTypeImmutable) {
			response.APIWithArgs(c, 400, 400, nil, "template.type_immutable", map[string]interface{}{
				"type": templateImmutableType(err),
			})
			return
		}
		response.API(c, 500, 500, nil, "替换模板失败："+err.Error())
		return
	}

	if result.Reload.HasFailures() {
		response.APIWithArgs(c, 500, 500, gin.H{
			"id":       templateID,
			"reloaded": result.Reload.Success,
			"failed":   result.Reload.Failed,
			"total":    result.Reload.Total,
		}, "template.replace.saved_but_reload_failed", map[string]interface{}{
			"failed": len(result.Reload.Failed),
			"total":  result.Reload.Total,
		})
		return
	}

	response.API(c, 200, 200, gin.H{"id": templateID, "reloaded": result.Reload.Success, "total": result.Reload.Total}, "替换成功")
}

// ImportTemplatePreview 上传并解析模板文件，但不落库。
// 它和 ReviewTemplate 类似，都是“先看结果、后决定是否确认保存”的预览型接口。
func ImportTemplatePreview(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.API(c, 400, 400, nil, "上传文件失败")
		return
	}

	if err := validateUploadFile(file); err != nil {
		response.API(c, 400, 400, nil, templateErrorMessage(c, err, i18n.KeyCommonInvalidParams))
		return
	}

	userType := c.PostForm("template_type")
	safeFilename := sanitizeFilename(file.Filename)
	tmpPath := filepath.Join(bootstrap.DataPath("uploads"), fmt.Sprintf("preview_%d_%s", time.Now().Unix(), safeFilename))

	if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
		response.API(c, 500, 500, nil, "创建目录失败："+err.Error())
		return
	}

	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		response.API(c, 500, 500, nil, "保存文件失败："+err.Error())
		return
	}

	cleanupDone := false
	defer func() {
		if !cleanupDone {
			if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
				logger.Warnf("[ImportTemplatePreview] 清理临时文件失败: %v", err)
			}
		}
	}()

	parsed, err := templatemodule.ParseTemplateFileWithType(tmpPath, userType)
	if err != nil {
		response.API(c, 400, 400, nil, templateErrorMessage(c, err, i18n.KeyCommonInvalidParams))
		return
	}

	if customName := strings.TrimSpace(c.PostForm("template_name")); customName != "" {
		parsed.TemplateName = customName
	}

	alarms := make([]templatemodule.NormalizedAlarm, len(parsed.Alarms))
	for i, a := range parsed.Alarms {
		na := templatemodule.NormalizedAlarm{
			PointID:     a.PointID,
			Name:        a.Name,
			Level:       a.Level,
			StartDelay:  a.StartDelay,
			EndDelay:    a.EndDelay,
			Description: a.Description,
		}
		if a.StartLowLimit != nil {
			na.StartLowLimit = *a.StartLowLimit
		}
		if a.StartHighLimit != nil {
			na.StartHighLimit = *a.StartHighLimit
		}
		if a.EndLowLimit != nil {
			na.EndLowLimit = *a.EndLowLimit
		}
		if a.EndHighLimit != nil {
			na.EndHighLimit = *a.EndHighLimit
		}
		alarms[i] = na
	}

	cleanupDone = true
	_ = os.Remove(tmpPath)

	response.API(c, 200, 200, ImportPreview{
		TemplateName: parsed.TemplateName,
		Description:  parsed.Description,
		TemplateType: parsed.TemplateType,
		Points:       parsed.Points,
		Alarms:       alarms,
	}, "解析成功")
}

func ImportTemplateConfirm(c *gin.Context) {
	var req ReplaceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	service := templatemodule.GetConfigService()
	templateID, err := service.CreateTemplate(templateapp.CreateTemplateInput{
		Name:         req.TemplateName,
		Description:  req.Description,
		TemplateType: req.TemplateType,
		Points:       req.Points,
		Alarms:       req.Alarms,
	})
	if err != nil {
		response.API(c, 500, 500, nil, "添加模板失败："+err.Error())
		return
	}

	response.API(c, 200, 200, gin.H{"id": templateID}, "导入成功")
}

func UploadTemplate(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		writeTemplateLegacyFail(c, apperrors.NewWithKey("TEMPLATE_UPLOAD_FAILED", "上传文件失败", nil), "上传文件失败")
		return
	}
	if err := validateUploadFile(file); err != nil {
		writeTemplateLegacyFail(c, err, i18n.KeyCommonInvalidParams)
		return
	}

	userType := c.PostForm("template_type")
	safeFilename := sanitizeFilename(file.Filename)
	tmpPath := filepath.Join(bootstrap.DataPath("uploads"), fmt.Sprintf("%d_%s", time.Now().Unix(), safeFilename))
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
		writeTemplateLegacyFail(c, errWithPrefix("创建目录失败", err), "创建目录失败")
		return
	}
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		writeTemplateLegacyFail(c, errWithPrefix("保存文件失败", err), "保存文件失败")
		return
	}

	cleanupDone := false
	defer func() {
		if !cleanupDone {
			if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
				logger.Warnf("[UploadTemplate] 清理临时文件失败: %v", err)
			}
		}
	}()

	parsed, err := templatemodule.ParseTemplateFileWithType(tmpPath, userType)
	if err != nil {
		writeTemplateLegacyFail(c, err, i18n.KeyCommonInvalidParams)
		return
	}

	templateName := c.PostForm("template_name")
	if templateName == "" {
		templateName = parsed.TemplateName
	}
	if templateName == "" || templateName == "导入的模板" {
		templateName = strings.TrimSuffix(strings.TrimSuffix(safeFilename, ".xlsx"), ".xls")
	}

	templateType := strings.TrimSpace(userType)
	if templateType == "" {
		templateType = strings.TrimSpace(parsed.TemplateType)
	}

	service := templatemodule.GetConfigService()
	templateID, err := service.CreateTemplate(templateapp.CreateTemplateInput{
		Name:         templateName,
		Description:  parsed.Description,
		TemplateType: templateType,
		Points:       parsed.Points,
		Alarms:       parsed.Alarms,
	})
	if err != nil {
		writeTemplateLegacyFail(c, errWithPrefix("保存模板失败", err), "保存模板失败")
		return
	}

	cleanupDone = true
	_ = os.Remove(tmpPath)
	c.JSON(200, gin.H{"result": gin.H{"result": "ok", "data": gin.H{"temp_id": int64(templateID), "message": response.Text(c, "模板导入成功")}}})
}

func GetSupportedTemplateTypes(c *gin.Context) {
	types := templatemodule.GetSupportedTemplateTypes()
	response.API(c, 200, 200, types, i18n.KeyCommonSuccess)
}

func isValidExcelFile(file *multipart.FileHeader) bool {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".xlsx" && ext != ".xls" {
		return false
	}

	f, err := file.Open()
	if err != nil {
		return false
	}
	defer f.Close()

	buffer := make([]byte, 512)
	if _, err := f.Read(buffer); err != nil {
		return false
	}

	mimeType := http.DetectContentType(buffer)
	validMimeTypes := map[string]bool{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
		"application/vnd.ms-excel": true,
		"application/zip":          true,
		"application/octet-stream": true,
	}
	return validMimeTypes[mimeType]
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	re := regexp.MustCompile(`[^\w\-.]`)
	filename = re.ReplaceAllString(filename, "_")
	if len(filename) > 200 {
		ext := filepath.Ext(filename)
		nameWithoutExt := filename[:len(filename)-len(ext)]
		if len(nameWithoutExt) > 200-len(ext) {
			nameWithoutExt = nameWithoutExt[:200-len(ext)]
		}
		filename = nameWithoutExt + ext
	}
	return filename
}

func validateUploadFile(file *multipart.FileHeader) error {
	if file.Size > maxFileSize {
		return apperrors.NewWithKey("TEMPLATE_UPLOAD_FILE_TOO_LARGE", "template.upload.file_too_large", map[string]interface{}{
			"limit_mb": maxFileSize / (1024 * 1024),
			"size":     file.Size,
		})
	}
	if !isValidExcelFile(file) {
		return apperrors.NewWithKey("TEMPLATE_UPLOAD_INVALID_FILE_TYPE", "template.upload.invalid_file_type", nil)
	}
	return nil
}

func localizeTemplateWarnings(c *gin.Context, warnings []templateapp.ReviewWarning) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, response.TextWithArgs(c, warning.Key, warning.Args))
	}
	return result
}

func joinTemplateWarnings(c *gin.Context, warnings []templateapp.ReviewWarning) string {
	separator := "；"
	if locale.IsEnglish(c) {
		separator = "; "
	}
	return strings.Join(localizeTemplateWarnings(c, warnings), separator)
}

func templateImmutableType(err error) string {
	const marker = "当前类型："
	raw := strings.TrimSpace(err.Error())
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(raw[idx+len(marker):])
}

func templateErrorMessage(c *gin.Context, err error, fallback string) string {
	if err == nil {
		return response.Text(c, fallback)
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		if strings.TrimSpace(appErr.Key) != "" {
			return response.TextWithArgs(c, appErr.Key, appErr.Args)
		}
		if strings.TrimSpace(appErr.Message) != "" {
			return response.Text(c, appErr.Message)
		}
	}
	if errors.Is(err, templateapp.ErrTemplateTypeImmutable) {
		return response.TextWithArgs(c, "template.type_immutable", map[string]interface{}{
			"type": templateImmutableType(err),
		})
	}
	if raw := strings.TrimSpace(err.Error()); raw != "" {
		return response.Text(c, raw)
	}
	return response.Text(c, fallback)
}

func writeTemplateLegacyFail(c *gin.Context, err error, fallback string) {
	c.JSON(200, gin.H{
		"result": gin.H{
			"result": "fail",
			"data": gin.H{
				"error": templateErrorMessage(c, err, fallback),
			},
		},
	})
}

func errWithPrefix(prefix string, err error) error {
	if err == nil {
		return errors.New(prefix)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
