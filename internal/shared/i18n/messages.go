package i18n

import (
	"fmt"
	"strings"
)

type messagePack struct {
	zh string
	en string
}

const (
	KeyCommonSuccess         = "common.success"
	KeyCommonInvalidParams   = "common.invalid_params"
	KeyCommonInvalidID       = "common.invalid_id"
	KeyCommonUnauthorized    = "common.unauthorized"
	KeyCommonTokenMissing    = "common.token_missing"
	KeyCommonTokenInvalid    = "common.token_invalid"
	KeyCommonForbidden       = "common.forbidden"
	KeyCommonInternalError   = "common.internal_error"
	KeyCommonNotFound        = "common.not_found"
	KeyCommonDeleteSuccess   = "common.delete_success"
	KeyCommonUpdateSuccess   = "common.update_success"
	KeyCommonSaveSuccess     = "common.save_success"
	KeyCommonCreateSuccess   = "common.create_success"
	KeyTemplateNotFound      = "template.not_found"
	KeyTemplateInvalidID     = "template.invalid_id"
	KeyMetricInvalidID       = "metric.invalid_id"
	KeyMetricNotFound        = "metric.not_found"
	KeyMetricQueryFailed     = "metric.query_failed"
	KeyMetricCreateFailed    = "metric.create_failed"
	KeyMetricUpdateFailed    = "metric.update_failed"
	KeyMetricDeleteFailed    = "metric.delete_failed"
	KeyMetricUpdateStateFail = "metric.update_state_failed"
	KeyMetricExprCheckFailed = "metric.expression_check_failed"
	KeyMetricExprCheckOK     = "metric.expression_check_passed"
	KeyMetricHelpVarFmt      = "metric.help.variable_format"
	KeyMetricHelpOperators   = "metric.help.operators"
	KeyMetricHelpFunctions   = "metric.help.functions"
	KeyMetricHelpExampleAvg  = "metric.help.example_avg"
	KeyMetricHelpExampleSum  = "metric.help.example_sum"
	KeyMetricHelpExampleRnd  = "metric.help.example_round"
	KeyMetricHelpBindTypes   = "metric.help.binding_types"
	KeyMetricHelpBadType     = "metric.help.unsupported_binding_type"
	KeyMetricHelpRule        = "metric.help.rule"
	KeyMetricErrVarMismatch  = "metric.error.variables_sources_mismatch"
	KeyMetricErrVarOrder     = "metric.error.source_var_order_invalid"
	KeyMetricErrPointMissing = "metric.error.source_point_not_found"
	KeyMetricErrVarUnbound   = "metric.error.variable_not_bound"
	KeyMetricErrMappedText   = "metric.error.mapped_text_invalid"
	KeyMetricErrExprEmpty    = "metric.error.empty_expression"
	KeyMetricErrNoVars       = "metric.error.no_variables"
	KeyMetricErrNameMissing  = "metric.error.metric_name_required"
	// Operation Log message keys
	KeyOperlogDeviceCreate   = "operlog.device.create"
	KeyOperlogDeviceUpdate   = "operlog.device.update"
	KeyOperlogDeviceDelete   = "operlog.device.delete"
	KeyOperlogDeviceStart    = "operlog.device.start"
	KeyOperlogDeviceStop     = "operlog.device.stop"
	KeyOperlogDeviceEnable   = "operlog.device.enable"
	KeyOperlogDeviceDisable  = "operlog.device.disable"
	KeyOperlogTemplateCreate = "operlog.template.create"
	KeyOperlogTemplateUpdate = "operlog.template.update"
	KeyOperlogTemplateDelete = "operlog.template.delete"
	KeyOperlogUserCreate     = "operlog.user.create"
	KeyOperlogUserUpdate     = "operlog.user.update"
	KeyOperlogUserDelete     = "operlog.user.delete"
	KeyOperlogSystemUpdate   = "operlog.system.update"
	KeyOperlogDeleted        = "operlog.deleted"
	KeyOperlogQueryFailed    = "operlog.query_failed"
	KeyOperlogDeleteFailed   = "operlog.delete_failed"
	// Linkage Operation Log message keys
	KeyOperlogLinkageActionCreate = "operlog.linkage.action.create"
	KeyOperlogLinkageActionUpdate = "operlog.linkage.action.update"
	KeyOperlogLinkageActionDelete = "operlog.linkage.action.delete"
	KeyOperlogLinkageRuleCreate   = "operlog.linkage.rule.create"
	KeyOperlogLinkageRuleUpdate   = "operlog.linkage.rule.update"
	KeyOperlogLinkageRuleDelete   = "operlog.linkage.rule.delete"
)

// CI 定制包导入相关文案。单独成块，避免影响上方既有常量块的 gofmt 对齐。
const (
	KeySystemCIImportFileRequired  = "system.ci_import_file_required"
	KeySystemCIImportFormatInvalid = "system.ci_import_format_invalid"
	KeySystemCIImportReadFailed    = "system.ci_import_read_failed"
	KeySystemCIImportFailed        = "system.ci_import_failed"
	KeySystemCIImportTooLarge      = "system.ci_import_too_large"
)

// CI 定制包导入失败的细分原因。单独成块，dict 条目用这些常量做 key，
// 避免并入既有常量块或换成更长的字符串字面量而触发 gofmt 重排。
const (
	KeySystemCIImportNoResources     = "system.ci_import_no_resources"
	KeySystemCIImportUnsupportedType = "system.ci_import_unsupported_type"
	KeySystemCIImportTotalSizeLimit  = "system.ci_import_total_size_limit"
	KeySystemCIImportEntryLimit      = "system.ci_import_entry_limit"
	KeySystemCIImportFileTooLarge    = "system.ci_import_file_too_large"
	KeySystemCIImportInvalidName     = "system.ci_import_invalid_name"
)

// History extremum message keys.
const (
	KeyHistoryNotificationQueryFailed  = "history.notification.query_failed"
	KeyHistoryNotificationDeleteFailed = "history.notification.delete_failed"
	KeyHistoryExtremumQueryFailed      = "history.extremum.query_failed"
	KeyHistoryExtremumDeleteFailed     = "history.extremum.delete_failed"
)

// 视频接入与 NVR 相关文案。
const (
	KeyVideoNVRNotFound       = "video.nvr_not_found"
	KeyVideoConnectionSuccess = "video.connection_success"
	KeyVideoConnectionFailed  = "video.connection_failed"
	KeyVideoStreamReady       = "video.stream_ready"
	KeyVideoOperationFailed   = "video.operation_failed"
)

// SNMP Trap 配置与测试相关文案。
const (
	KeyNorthboundSNMPTrapLoadFailed  = "northbound.snmp_trap_load_failed"
	KeyNorthboundSNMPTrapSaveFailed  = "northbound.snmp_trap_save_failed"
	KeyNorthboundSNMPTrapInvalid     = "northbound.snmp_trap_invalid"
	KeyNorthboundSNMPTrapTestFailed  = "northbound.snmp_trap_test_failed"
	KeyNorthboundSNMPTrapTestSuccess = "northbound.snmp_trap_test_success"
)

// 配置还原点相关文案。单独成块，避免影响上方既有常量块的 gofmt 对齐。
const (
	KeyMaintenanceRestorePointCreateFailed = "maintenance.restore_point_create_failed"
	KeyMaintenanceRestorePointInvalidName  = "maintenance.restore_point_invalid_name"
	KeyMaintenanceRestorePointListFailed   = "maintenance.restore_point_list_failed"
	KeyMaintenanceConfigRestoreFailed      = "maintenance.config_restore_failed"
)

// 版本升级与恢复出厂相关文案。
const (
	KeySystemUpgradeSuccess           = "system.upgrade_success"
	KeySystemUpgradeFailedGetFile     = "system.upgrade_failed_get_file"
	KeySystemUpgradeFailedExePath     = "system.upgrade_failed_exe_path"
	KeySystemUpgradeFailedCreateFile  = "system.upgrade_failed_create_file"
	KeySystemUpgradeFailedWrite       = "system.upgrade_failed_write"
	KeySystemUpgradeFailedRename      = "system.upgrade_failed_rename"
	KeySystemUpgradeInvalidPackage    = "system.upgrade_invalid_package"
	KeySystemUpgradeBackupFailed      = "system.upgrade_backup_failed"
	KeySystemVersionBackupListFailed  = "system.version_backup_list_failed"
	KeySystemVersionBackupInvalidName = "system.version_backup_invalid_name"
	KeySystemVersionRollbackFailed    = "system.version_rollback_failed"
	KeySystemVersionRollbackSuccess   = "system.version_rollback_success"
	KeySystemFactoryBaselineMissing   = "system.factory_baseline_missing"
	KeySystemFactoryResetFailed       = "system.factory_reset_failed"
)

var dictionary = map[string]messagePack{
	KeyCommonSuccess:                   {zh: "success", en: "success"},
	KeyCommonInvalidParams:             {zh: "参数错误", en: "Invalid parameters"},
	KeyCommonInvalidID:                 {zh: "无效的ID", en: "Invalid ID"},
	KeyCommonUnauthorized:              {zh: "未授权", en: "Unauthorized"},
	KeyCommonTokenMissing:              {zh: "未提供Token", en: "Token is required"},
	KeyCommonTokenInvalid:              {zh: "Token无效或已过期", en: "Token is invalid or expired"},
	KeyCommonForbidden:                 {zh: "权限不足", en: "Permission denied"},
	KeyCommonInternalError:             {zh: "服务器内部错误", en: "Internal server error"},
	KeyCommonNotFound:                  {zh: "资源不存在", en: "Resource not found"},
	KeyCommonDeleteSuccess:             {zh: "删除成功", en: "Deleted successfully"},
	KeyCommonUpdateSuccess:             {zh: "更新成功", en: "Updated successfully"},
	KeyCommonSaveSuccess:               {zh: "保存成功", en: "Saved successfully"},
	KeyCommonCreateSuccess:             {zh: "创建成功", en: "Created successfully"},
	KeyHistoryNotificationQueryFailed:  {zh: "查询通知记录失败", en: "Failed to query notification history"},
	KeyHistoryNotificationDeleteFailed: {zh: "删除通知记录失败", en: "Failed to delete notification history"},
	KeyTemplateNotFound:                {zh: "模板不存在", en: "Template not found"},
	KeyTemplateInvalidID:               {zh: "无效的模板ID", en: "Invalid template ID"},
	"template.review.warn.referenced":  {zh: "该模板当前被 {{count}} 个设备引用，确认后将触发配置热同步", en: "This template is referenced by {{count}} devices. Confirming will trigger hot-sync."},
	"template.review.warn.point_count_changed": {
		zh: "测点数量将从 {{from}} 调整为 {{to}}",
		en: "Point count will change from {{from}} to {{to}}",
	},
	"template.review.warn.alarm_count_changed": {
		zh: "告警数量将从 {{from}} 调整为 {{to}}",
		en: "Alarm count will change from {{from}} to {{to}}",
	},
	"template.review.warn.name_changed": {
		zh: "模板名称将从 {{from}} 更新为 {{to}}",
		en: "Template name will be changed from {{from}} to {{to}}",
	},
	"template.review.warn.ok":                 {zh: "模板数据检查通过，确认后将更新模板配置", en: "Template validation passed. Confirm to apply the template update."},
	"template.delete.referenced":              {zh: "该模板被 {{count}} 个设备引用，无法删除", en: "This template is referenced by {{count}} devices and cannot be deleted"},
	"template.delete.referenced_with_devices": {zh: "该模板被 {{count}} 个设备引用（{{devices}}），无法删除", en: "This template is referenced by {{count}} devices ({{devices}}) and cannot be deleted"},
	"template.replace.saved_but_reload_failed": {
		zh: "模板已保存，但有 {{failed}}/{{total}} 个运行中设备重启失败",
		en: "Template saved, but restart failed for {{failed}}/{{total}} running devices",
	},
	"template.type_immutable": {
		zh: "模板类型不允许修改，当前类型：{{type}}",
		en: "Template type cannot be changed. Current type: {{type}}",
	},
	"template.upload.file_too_large": {
		zh: "文件大小超过限制（最大{{limit_mb}}MB），当前大小：{{size}}字节",
		en: "File size exceeds the limit (max {{limit_mb}}MB), current size: {{size}} bytes",
	},
	"template.upload.invalid_file_type": {
		zh: "不支持的文件类型，仅支持 .xlsx 和 .xls 格式",
		en: "Unsupported file type. Only .xlsx and .xls are supported",
	},
	KeyMetricInvalidID:                   {zh: "无效的指标ID", en: "Invalid metric ID"},
	KeyMetricNotFound:                    {zh: "指标不存在", en: "Metric not found"},
	KeyMetricQueryFailed:                 {zh: "查询指标失败", en: "Failed to query metrics"},
	KeyMetricCreateFailed:                {zh: "创建指标失败", en: "Failed to create metric"},
	KeyMetricUpdateFailed:                {zh: "更新指标失败", en: "Failed to update metric"},
	KeyMetricDeleteFailed:                {zh: "删除指标失败", en: "Failed to delete metric"},
	KeyMetricUpdateStateFail:             {zh: "设置指标启用状态失败", en: "Failed to update metric status"},
	KeyMetricExprCheckFailed:             {zh: "表达式校验失败", en: "Expression validation failed"},
	KeyMetricExprCheckOK:                 {zh: "校验通过", en: "Validation passed"},
	KeyMetricHelpVarFmt:                  {zh: "变量格式：a1、a2、a3 ...", en: "Variable format: a1, a2, a3 ..."},
	KeyMetricHelpOperators:               {zh: "支持运算：+ - * / % ()", en: "Operators: + - * / % ()"},
	KeyMetricHelpFunctions:               {zh: "支持函数：avg sum min max abs round", en: "Functions: avg sum min max abs round"},
	KeyMetricHelpExampleAvg:              {zh: "示例：avg(a1, a2, a3)", en: "Example: avg(a1, a2, a3)"},
	KeyMetricHelpExampleSum:              {zh: "示例：sum(a1, a2, a3)", en: "Example: sum(a1, a2, a3)"},
	KeyMetricHelpExampleRnd:              {zh: "示例：round((a1-a2)/a3, 2)", en: "Example: round((a1-a2)/a3, 2)"},
	KeyMetricHelpBindTypes:               {zh: "支持绑定类型：当前值、设备当前告警数、测点当前告警数", en: "Supported binding types: current value, device active alarm count, point active alarm count"},
	KeyMetricHelpBadType:                 {zh: "不支持绑定类型：值映射（字符串不能参与数值表达式）", en: "Unsupported binding type: mapped value (text cannot be used in numeric expressions)"},
	KeyMetricHelpRule:                    {zh: "规则：表达式中的变量必须与已选择的测点一一对应", en: "Rule: every variable in expression must be bound to a selected source"},
	KeyMetricErrVarMismatch:              {zh: "表达式中的变量数量与绑定值数量不一致", en: "Variable count in expression does not match the source bindings"},
	KeyMetricErrVarOrder:                 {zh: "绑定值变量顺序无效，请按 a1、a2、a3 顺序绑定", en: "Source variable order is invalid, expected a1, a2, a3 ..."},
	KeyMetricErrPointMissing:             {zh: "绑定的设备测点不存在", en: "Bound source point does not exist"},
	KeyMetricErrVarUnbound:               {zh: "表达式中的变量存在未绑定项", en: "One or more variables in expression are not bound"},
	KeyMetricErrMappedText:               {zh: "值映射为字符串类型，不能用于指标数值表达式", en: "Mapped value is text-based and cannot be used in numeric metric expressions"},
	KeyMetricErrExprEmpty:                {zh: "表达式不能为空", en: "Expression cannot be empty"},
	KeyMetricErrNoVars:                   {zh: "表达式中未找到变量，请使用 a1、a2 等变量", en: "No variables found in expression, please use a1, a2 ..."},
	KeyMetricErrNameMissing:              {zh: "指标名称不能为空", en: "Metric name is required"},
	"auth.invalid_credentials":           {zh: "用户名或密码错误", en: "Invalid username or password"},
	"auth.user_disabled":                 {zh: "账户已被禁用", en: "Account is disabled"},
	"auth.user_locked":                   {zh: "账户已被锁定，请稍后再试", en: "Account is locked, please try again later"},
	"auth.login_time_restricted":         {zh: "当前时间规则未启用，禁止登录", en: "Login is not allowed at the current time"},
	"auth.token_generate_failed":         {zh: "生成Token失败", en: "Failed to generate token"},
	"auth.logout_success":                {zh: "登出成功", en: "Logged out successfully"},
	"auth.get_current_user_failed":       {zh: "获取用户信息失败", en: "Failed to get current user"},
	"auth.list_users_failed":             {zh: "获取用户列表失败", en: "Failed to list users"},
	"auth.create_user_failed":            {zh: "创建用户失败", en: "Failed to create user"},
	"auth.invalid_user_id":               {zh: "无效的用户ID", en: "Invalid user ID"},
	"auth.update_user_failed":            {zh: "更新用户失败", en: "Failed to update user"},
	"auth.cannot_delete_self":            {zh: "不能删除自己", en: "You cannot delete yourself"},
	"auth.delete_user_failed":            {zh: "删除用户失败", en: "Failed to delete user"},
	"auth.default_group_immutable":       {zh: "默认用户组不允许修改", en: "Default user group cannot be modified"},
	"auth.user_group_name_required":      {zh: "用户组名称不能为空", en: "User group name is required"},
	"auth.user_group_name_duplicated":    {zh: "用户组名称已存在", en: "User group name already exists"},
	"auth.user_group_in_use":             {zh: "该用户组正在被用户引用，无法删除", en: "This user group is in use and cannot be deleted"},
	"auth.update_status_failed":          {zh: "更新状态失败", en: "Failed to update status"},
	"auth.update_status_success":         {zh: "状态更新成功", en: "Status updated successfully"},
	"auth.unlock_user_failed":            {zh: "解锁用户失败", en: "Failed to unlock user"},
	"auth.unlock_user_success":           {zh: "解锁成功", en: "Unlocked successfully"},
	"auth.wrong_old_password":            {zh: "原密码错误", en: "Old password is incorrect"},
	"auth.password_too_simple":           {zh: "密码过于简单：至少8位且需同时包含字母和数字", en: "Password is too simple: at least 8 characters with both letters and digits"},
	"auth.change_password_failed":        {zh: "更新密码失败", en: "Failed to change password"},
	"auth.change_password_success":       {zh: "密码修改成功", en: "Password changed successfully"},
	"system.invalid_request":             {zh: "无效的请求数据", en: "Invalid request payload"},
	"system.invalid_ip":                  {zh: "无效的IP地址", en: "Invalid IP address"},
	"system.invalid_network":             {zh: "无效的系统网卡", en: "Invalid system network"},
	"system.network_saved":               {zh: "网络配置已保存", en: "Network configuration saved"},
	"system.network_unsupported":         {zh: "当前系统不支持修改网络配置", en: "Network configuration is not supported on this system"},
	"system.network_apply_failed":        {zh: "网络配置应用失败", en: "Failed to apply the network configuration"},
	"system.network_busy":                {zh: "网络配置下发中，请稍后再试", en: "A network configuration is already being applied, please retry later"},
	"system.dualscreen_saved":            {zh: "双屏配置已保存", en: "Dual screen configuration saved"},
	"system.get_dualscreen_failed":       {zh: "获取双屏配置失败", en: "Failed to get dual screen configuration"},
	"system.save_dualscreen_failed":      {zh: "保存双屏配置失败", en: "Failed to save dual screen configuration"},
	"system.system_name_saved":           {zh: "系统名称已保存", en: "System name saved"},
	"system.system_network_saved":        {zh: "系统网卡已保存", en: "System network saved"},
	"system.save_system_name_failed":     {zh: "保存系统名称失败", en: "Failed to save system name"},
	"system.save_system_network_failed":  {zh: "保存系统网卡失败", en: "Failed to save system network"},
	"system.upgrade_success":             {zh: "升级成功，正在重启系统", en: "Upgrade successful, restarting system"},
	"system.upgrade_failed_get_file":     {zh: "获取上传文件失败", en: "Failed to get uploaded file"},
	"system.upload_get_file_failed":      {zh: "获取上传文件失败", en: "Failed to get uploaded file"},
	"system.upload_save_failed":          {zh: "文件保存失败", en: "Failed to save file"},
	"system.upload_unsupported_type":     {zh: "仅支持上传 png/jpg/jpeg/gif/webp 图片", en: "Only png/jpg/jpeg/gif/webp images are supported"},
	"system.image_delete_failed":         {zh: "删除图片失败", en: "Failed to delete image"},
	"system.image_in_use":                {zh: "该图片正被界面配置引用，无法删除", en: "This image is referenced by a UI config and cannot be deleted"},
	"system.image_ref_scan_failed":       {zh: "图片引用检查失败，请稍后重试", en: "Failed to check image references, please retry later"},
	"system.image_ref_clear_failed":      {zh: "清理图片引用失败，请稍后重试", en: "Failed to clear image references, please retry later"},
	"system.ci_import_file_required":     {zh: "请上传 CI 定制文件", en: "Please upload a CI package"},
	"system.ci_import_format_invalid":    {zh: "仅支持 .tar.xz 或 .tar.gz 文件", en: "Only .tar.xz or .tar.gz archives are supported"},
	"system.ci_import_read_failed":       {zh: "读取上传文件失败", en: "Failed to read the uploaded file"},
	"system.ci_import_failed":            {zh: "CI 定制文件导入失败", en: "Failed to import the CI package"},
	"system.ci_import_too_large":         {zh: "CI 定制包超过 64MB 体积上限", en: "The CI package exceeds the 64MB upload limit"},
	KeySystemCIImportNoResources:         {zh: "CI 包内没有可导入的资源", en: "The CI package contains no importable resources"},
	KeySystemCIImportUnsupportedType:     {zh: "CI 包含不支持的文件类型: {{name}}", en: "The CI package contains an unsupported file type: {{name}}"},
	KeySystemCIImportTotalSizeLimit:      {zh: "CI 包解压后总体积超过上限", en: "The extracted size of the CI package exceeds the limit"},
	KeySystemCIImportEntryLimit:          {zh: "CI 包条目数超过上限 {{limit}}", en: "The CI package contains too many entries (limit {{limit}})"},
	KeySystemCIImportFileTooLarge:        {zh: "CI 包内文件超过体积上限: {{name}}", en: "A file in the CI package exceeds the size limit: {{name}}"},
	KeySystemCIImportInvalidName:         {zh: "CI 包内文件名非法: {{name}}", en: "Invalid file name in the CI package: {{name}}"},
	"system.upgrade_failed_exe_path":     {zh: "获取程序运行路径失败", en: "Failed to get executable path"},
	"system.upgrade_failed_create_file":  {zh: "无法创建临时文件", en: "Failed to create temporary file"},
	"system.upgrade_failed_write":        {zh: "写入文件流失败", en: "Failed to write file stream"},
	"system.upgrade_failed_rename":       {zh: "覆盖程序二进制失败", en: "Failed to overwrite executable binary"},
	KeySystemUpgradeInvalidPackage:       {zh: "升级包不是本机架构的可执行文件", en: "Upgrade package is not an executable for this architecture"},
	KeySystemUpgradeBackupFailed:         {zh: "升级前备份当前版本失败", en: "Failed to back up the current version before upgrade"},
	KeySystemVersionBackupListFailed:     {zh: "获取版本备份列表失败", en: "Failed to list version backups"},
	KeySystemVersionBackupInvalidName:    {zh: "版本备份名非法", en: "Invalid version backup name"},
	KeySystemVersionRollbackFailed:       {zh: "回滚版本失败", en: "Failed to roll back version"},
	KeySystemVersionRollbackSuccess:      {zh: "回滚成功，正在重启系统", en: "Rollback successful, restarting system"},
	KeySystemFactoryBaselineMissing:      {zh: "出厂基线缺失，无法恢复出厂设置", en: "Factory baseline missing, cannot reset to factory settings"},
	KeySystemFactoryResetFailed:          {zh: "恢复出厂设置失败", en: "Failed to reset to factory settings"},
	"system.diagnosis.invalid_target":    {zh: "目标IP或域名格式无效", en: "Invalid target IP or domain format"},
	"system.diagnosis.tool_not_found":    {zh: "未在系统上检测到诊断工具", en: "Diagnostic tool not found on system"},
	"system.diagnosis.execution_timeout": {zh: "诊断执行超时，请稍后重试", en: "Diagnosis execution timed out, please try again"},
	"system.diagnosis.execution_failed":  {zh: "诊断命令执行失败", en: "Failed to execute diagnostic command"},
	"system.invalid_time_format":         {zh: "无效的时间格式", en: "Invalid time format"},
	"system.time_saved":                  {zh: "时间配置已保存", en: "Time configuration saved"},
	"system.time_sync_failed":            {zh: "时间同步失败", en: "Failed to sync time"},
	"system.time_sync_success":           {zh: "时间同步成功", en: "Time synced successfully"},
	"system.ntp_saved":                   {zh: "NTP 服务器地址已保存", en: "NTP server saved"},
	"system.ntp_invalid_address":         {zh: "NTP 服务器地址无效", en: "Invalid NTP server address"},
	"system.ntp_not_configured":          {zh: "未配置 NTP 服务器地址", en: "NTP server is not configured"},
	"system.ntp_save_failed":             {zh: "保存 NTP 服务器地址失败", en: "Failed to save NTP server"},
	"system.time_set_failed":             {zh: "设置系统时间失败", en: "Failed to set system time"},
	"system.timezone_set_failed":         {zh: "设置时区失败", en: "Failed to set timezone"},
	"system.hwclock_write_failed":        {zh: "硬件时钟写入失败，断电后时间可能回退", en: "Failed to write hardware clock; time may be lost after power off"},
	"system.query_time_config_failed":    {zh: "获取时间配置失败", en: "Failed to query time configuration"},
	"system.unsupported_ui_config_name":  {zh: "不支持的界面配置名称", en: "Unsupported UI config name"},
	"system.query_ui_config_failed":      {zh: "查询界面配置失败", en: "Failed to query UI config"},
	"system.save_ui_config_failed":       {zh: "保存界面配置失败", en: "Failed to save UI config"},
	"system.list_time_rules_failed":      {zh: "获取时间规则失败", en: "Failed to list time rules"},
	"system.save_time_rule_failed":       {zh: "保存时间规则失败", en: "Failed to save time rule"},
	"system.delete_time_rule_failed":     {zh: "删除时间规则失败", en: "Failed to delete time rule"},
	"system.default_time_rule_immutable": {zh: "默认时间规则不允许修改", en: "Default time rule cannot be modified"},
	"system.time_rule_in_use":            {zh: "该时间规则正在被用户引用，无法删除", en: "This time rule is in use and cannot be deleted"},
	"device.invalid_protocol":            {zh: "无效的协议类型", en: "Invalid protocol type"},
	"device.copy_gpio_conflict":          {zh: "本地 IO 设备不能直接复制，请新建并选择新的控制器", en: "Local IO devices cannot be copied directly; create a new device and select a controller"},
	"设备引用不存在或已失效":                        {zh: "设备引用不存在或已失效", en: "Device reference does not exist or is no longer valid"},
	"设备引用不能为空":                           {zh: "设备引用不能为空", en: "Device reference is required"},
	"设备引用与协议不匹配":                         {zh: "设备引用与协议不匹配", en: "Device reference does not match the selected protocol"},
	"固定参数不允许修改":                          {zh: "固定参数不允许修改", en: "Fixed parameters cannot be modified"},
	"模板要求的本地 IO 通道不存在":                   {zh: "模板要求的本地 IO 通道不存在", en: "A local IO channel required by the template does not exist"},
	"本地 IO 通道方向不匹配":                      {zh: "本地 IO 通道方向不匹配", en: "Local IO channel direction does not match"},
	"DIDO类型无效":                           {zh: "DIDO类型无效", en: "Invalid DIDO direction"},
	"DIDO接口不存在":                          {zh: "DIDO接口不存在", en: "DIDO interface does not exist"},
	"DIDO接口方向不匹配":                        {zh: "DIDO接口方向不匹配", en: "DIDO interface direction does not match"},
	"别名不能包含冒号或逗号":                        {zh: "别名不能包含冒号或逗号", en: "Aliases cannot contain colons or commas"},
	"无效的数值":                              {zh: "无效的数值", en: "Invalid numeric value"},
	"无效的布尔值":                             {zh: "无效的布尔值", en: "Invalid boolean value"},
	"无效的字符串值":                            {zh: "无效的字符串值", en: "Invalid string value"},
	"该测点不支持写入":                           {zh: "该测点不支持写入", en: "This point does not support writing"},
	"查询测点失败":                             {zh: "查询测点失败", en: "Failed to query device points"},
	"复制设备失败":                             {zh: "复制设备失败", en: "Failed to copy device"},
	"写入失败":                               {zh: "写入失败", en: "Failed to write value"},
	"生成配置备份失败":                           {zh: "生成配置备份失败", en: "Failed to generate configuration backup"},
	"请上传配置备份文件":                          {zh: "请上传配置备份文件", en: "Please upload a configuration backup file"},
	"读取上传文件失败":                           {zh: "读取上传文件失败", en: "Failed to read the uploaded file"},
	"创建临时文件失败":                           {zh: "创建临时文件失败", en: "Failed to create temporary file"},
	"保存上传文件失败":                           {zh: "保存上传文件失败", en: "Failed to save the uploaded file"},
	"配置还原失败":                             {zh: "配置还原失败", en: "Failed to restore configuration"},
	"保存邮件服务器设置失败":                        {zh: "保存邮件服务器设置失败", en: "Failed to save email server settings"},
	"保存4G服务器设置失败":                        {zh: "保存4G服务器设置失败", en: "Failed to save 4G server settings"},
	"保存Webhook代理服务器设置失败":                 {zh: "保存Webhook代理服务器设置失败", en: "Failed to save Webhook proxy server settings"},
	"为了保证边缘端渲染流畅，最多允许同时探索 20 个测点": {zh: "为了保证边缘端渲染流畅，最多允许同时探索 20 个测点", en: "To keep edge rendering smooth, at most 20 points can be explored at the same time"},
	"mib_name 不能为空":             {zh: "mib_name 不能为空", en: "mib_name is required"},
	"mib_name 只能包含字母、数字及 . _ -": {zh: "mib_name 只能包含字母、数字及 . _ -", en: "mib_name may only contain letters, digits and . _ -"},
	"请求参数错误":                    {zh: "请求参数错误", en: "Invalid request parameters"},
	"打开文件失败":                    {zh: "打开文件失败", en: "Failed to open file"},
	"解析测点表头失败":                  {zh: "解析测点表头失败", en: "Failed to parse point headers"},
	"模板数据反序列化失败":                {zh: "模板数据反序列化失败", en: "Failed to deserialize template data"},
	"是必填项":                      {zh: "是必填项", en: "is required"},
	"必须是整数":                     {zh: "必须是整数", en: "must be an integer"},
	"必须是以下值之一":                  {zh: "必须是以下值之一", en: "must be one of the following values"},
	"必须是布尔值 (true/false 或 0/1)": {zh: "必须是布尔值 (true/false 或 0/1)", en: "must be a boolean value (true/false or 0/1)"},
	"IP地址格式无效":                  {zh: "IP地址格式无效", en: "Invalid IP address format"},
	"串口路径不能为空":                  {zh: "串口路径不能为空", en: "Serial port path is required"},
	"串口路径格式无效 (Linux: /dev/ttyUSB0, Windows: COM1)": {
		zh: "串口路径格式无效 (Linux: /dev/ttyUSB0, Windows: COM1)",
		en: "Invalid serial port path format (Linux: /dev/ttyUSB0, Windows: COM1)",
	},
	"SNMPv3 为必填项":     {zh: "SNMPv3 为必填项", en: "SNMPv3 user is required"},
	"SNMPv3 安全级别为必填项": {zh: "SNMPv3 安全级别为必填项", en: "SNMPv3 security level is required"},
	"SNMPv3 认证协议为必填项": {zh: "SNMPv3 认证协议为必填项", en: "SNMPv3 authentication protocol is required"},
	"SNMPv3 认证密码为必填项": {zh: "SNMPv3 认证密码为必填项", en: "SNMPv3 authentication password is required"},
	"SNMPv3 加密协议为必填项": {zh: "SNMPv3 加密协议为必填项", en: "SNMPv3 privacy protocol is required"},
	"SNMPv3 加密密码为必填项": {zh: "SNMPv3 加密密码为必填项", en: "SNMPv3 privacy password is required"},
	"SNMPv3 安全级别必须是 noAuthNoPriv/authNoPriv/authPriv 之一": {
		zh: "SNMPv3 安全级别必须是 noAuthNoPriv/authNoPriv/authPriv 之一",
		en: "SNMPv3 security level must be one of noAuthNoPriv/authNoPriv/authPriv",
	},
	"control_level 仅支持 1~3": {zh: "control_level 仅支持 1~3", en: "control_level only supports 1~3"},
	"上传文件失败":                {zh: "上传文件失败", en: "Failed to upload file"},
	"保存告警设置失败":              {zh: "保存告警设置失败", en: "Failed to save alarm settings"},
	"保存告警配置失败":              {zh: "保存告警配置失败", en: "Failed to save alarm configuration"},
	"保存文件失败":                {zh: "保存文件失败", en: "Failed to save file"},
	"保存模板失败":                {zh: "保存模板失败", en: "Failed to save template"},
	"保存维护设置失败":              {zh: "保存维护设置失败", en: "Failed to save maintenance settings"},
	"保存配置失败":                {zh: "保存配置失败", en: "Failed to save configuration"},
	"停止成功":                  {zh: "停止成功", en: "Stopped successfully"},
	"停止采集线程超时，请稍后重试删除":      {zh: "停止采集线程超时，请稍后重试删除", en: "Timed out stopping collector thread, please retry deletion later"},
	"停止采集线程超时，请重试":          {zh: "停止采集线程超时，请重试", en: "Timed out stopping collector thread, please retry"},
	"内置模板不允许删除":             {zh: "内置模板不允许删除", en: "Built-in template cannot be deleted"},
	"内置模板不允许替换":             {zh: "内置模板不允许替换", en: "Built-in template cannot be replaced"},
	"创建目录失败":                {zh: "创建目录失败", en: "Failed to create directory"},
	"删除告警配置失败":              {zh: "删除告警配置失败", en: "Failed to delete alarm configuration"},
	"删除模板失败":                {zh: "删除模板失败", en: "Failed to delete template"},
	"删除设备失败":                {zh: "删除设备失败", en: "Failed to delete device"},
	"删除配置失败":                {zh: "删除配置失败", en: "Failed to delete configuration"},
	"参数已保存，但采集器重启失败，请手动重启设备": {zh: "参数已保存，但采集器重启失败，请手动重启设备", en: "Parameters saved, but collector restart failed, please restart device manually"},
	"已启用":     {zh: "已启用", en: "Enabled"},
	"已禁用":     {zh: "已禁用", en: "Disabled"},
	"启动设备失败":  {zh: "启动设备失败", en: "Failed to start device"},
	"启动成功":    {zh: "启动成功", en: "Started successfully"},
	"导入成功":    {zh: "导入成功", en: "Imported successfully"},
	"无效测点ID":  {zh: "无效测点ID", en: "Invalid point ID"},
	"无效的告警ID": {zh: "无效的告警ID", en: "Invalid alarm ID"},
	"无效的测点ID": {zh: "无效的测点ID", en: "Invalid point ID"},
	"无效的设备ID": {zh: "无效的设备ID", en: "Invalid device ID"},
	"无效设备ID":  {zh: "无效设备ID", en: "Invalid device ID"},
	"更新成功，但采集器重启失败，请手动重启设备": {zh: "更新成功，但采集器重启失败，请手动重启设备", en: "Updated successfully, but collector restart failed, please restart device manually"},
	"更新设备参数失败":   {zh: "更新设备参数失败", en: "Failed to update device parameters"},
	"更新设备失败":     {zh: "更新设备失败", en: "Failed to update device"},
	"更新设备状态失败":   {zh: "更新设备状态失败", en: "Failed to update device status"},
	"替换成功":       {zh: "替换成功", en: "Replaced successfully"},
	"替换模板失败":     {zh: "替换模板失败", en: "Failed to replace template"},
	"未提供任何可更新字段": {zh: "未提供任何可更新字段", en: "No fields provided for update"},
	"查询历史数据失败":   {zh: "查询历史数据失败", en: "Failed to query history data"},
	"查询告警数据失败":   {zh: "查询告警数据失败", en: "Failed to query alarm data"},
	"查询告警记录失败":   {zh: "查询告警记录失败", en: "Failed to query alarm records"},
	"查询告警设置失败":   {zh: "查询告警设置失败", en: "Failed to query alarm settings"},
	"查询告警配置失败":   {zh: "查询告警配置失败", en: "Failed to query alarm configuration"},
	"查询数据失败":     {zh: "查询数据失败", en: "Failed to query data"},
	"查询模板失败":     {zh: "查询模板失败", en: "Failed to query template"},
	"查询登录日志失败":   {zh: "查询登录日志失败", en: "Failed to query login logs"},
	"查询维护设置失败":   {zh: "查询维护设置失败", en: "Failed to query maintenance settings"},
	"查询设备测点失败":   {zh: "查询设备测点失败", en: "Failed to query device points"},
	"查询设备配置失败":   {zh: "查询设备配置失败", en: "Failed to query device configuration"},
	"模板检查失败":     {zh: "模板检查失败", en: "Template validation failed"},
	"添加模板失败":     {zh: "添加模板失败", en: "Failed to add template"},
	"添加设备失败":     {zh: "添加设备失败", en: "Failed to add device"},
	"获取历史数据失败":   {zh: "获取历史数据失败", en: "Failed to get history data"},
	"获取设备列表失败":   {zh: "获取设备列表失败", en: "Failed to get device list"},
	"获取设备参数失败":   {zh: "获取设备参数失败", en: "Failed to get device parameters"},
	"获取设备详情失败":   {zh: "获取设备详情失败", en: "Failed to get device details"},
	"获取选择器值失败":   {zh: "获取选择器值失败", en: "Failed to get selector values"},
	"获取选择器测点失败":  {zh: "获取选择器测点失败", en: "Failed to get selector points"},
	"解析成功":       {zh: "解析成功", en: "Parsed successfully"},
	"设备不存在":      {zh: "设备不存在", en: "Device not found"},
	"设备已在运行中":    {zh: "设备已在运行中", en: "Device is already running"},
	"设备或测点不存在":   {zh: "设备或测点不存在", en: "Device or point not found"},
	"设备未在运行":     {zh: "设备未在运行", en: "Device is not running"},
	"设备采集线程已在运行": {zh: "设备采集线程已在运行", en: "Device collector thread is already running"},
	"重置配置失败":     {zh: "重置配置失败", en: "Failed to reset configuration"},
	"模板导入成功":     {zh: "模板导入成功", en: "Template imported successfully"},

	"alarm.notification_rule_invalid":             {zh: "告警通知规则参数无效", en: "Invalid alarm notification rule"},
	"alarm.notification_rule_duplicate":           {zh: "告警通知规则重复", en: "Duplicate alarm notification rule"},
	"alarm.notification_rule_time_invalid":        {zh: "告警通知规则时间规则无效", en: "Invalid time rule for alarm notification rule"},
	"alarm.notification_rule_group_invalid":       {zh: "告警通知规则用户组无效", en: "Invalid user group for alarm notification rule"},
	"alarm.notification_rule_method_invalid":      {zh: "告警通知方式无效", en: "Invalid notification method for alarm notification rule"},
	"alarm.list_point_notification_rules_failed":  {zh: "获取告警点通知规则失败", en: "Failed to list point notification rules"},
	"alarm.save_point_notification_rules_failed":  {zh: "保存告警点通知规则失败", en: "Failed to save point notification rules"},
	"alarm.list_device_notification_rules_failed": {zh: "获取设备通知规则失败", en: "Failed to list device notification rules"},
	"alarm.save_device_notification_rules_failed": {zh: "保存设备通知规则失败", en: "Failed to save device notification rules"},
	"alarm.record_not_found":                      {zh: "告警记录不存在", en: "Alarm record not found"},
	"alarm.confirm_failed":                        {zh: "确认告警失败", en: "Failed to confirm the alarm"},
	"alarm.ignore_failed":                         {zh: "忽略告警失败", en: "Failed to ignore the alarm"},
	"alarm.auto_confirm_failed":                   {zh: "保存告警自动确认设置失败", en: "Failed to save the alarm auto-confirm settings"},
	KeyNorthboundSNMPTrapLoadFailed:               {zh: "获取 SNMP Trap 设置失败", en: "Failed to load SNMP Trap settings"},
	KeyNorthboundSNMPTrapSaveFailed:               {zh: "保存 SNMP Trap 设置失败", en: "Failed to save SNMP Trap settings"},
	KeyNorthboundSNMPTrapInvalid:                  {zh: "SNMP Trap 设置无效", en: "Invalid SNMP Trap settings"},
	KeyNorthboundSNMPTrapTestFailed:               {zh: "SNMP Trap 测试发送失败", en: "Failed to send the SNMP Trap test"},
	KeyNorthboundSNMPTrapTestSuccess:              {zh: "SNMP Trap 测试已发送", en: "SNMP Trap test sent"},
	KeyVideoNVRNotFound:                           {zh: "NVR 不存在", en: "NVR not found"},
	KeyVideoConnectionSuccess:                     {zh: "NVR 连接成功", en: "NVR connected successfully"},
	KeyVideoConnectionFailed:                      {zh: "NVR 连接失败，请检查地址、端口和账号密码", en: "NVR connection failed; check address, port, username and password"},
	KeyVideoStreamReady:                           {zh: "视频流已就绪", en: "Video stream is ready"},
	KeyVideoOperationFailed:                       {zh: "视频接入操作失败", en: "Video operation failed"},
	// Operation Log keys
	KeyOperlogDeviceCreate:        {zh: "创建设备 {{name}}", en: "Created device {{name}}"},
	KeyOperlogDeviceUpdate:        {zh: "修改设备 {{name}}", en: "Updated device {{name}}"},
	KeyOperlogDeviceDelete:        {zh: "删除设备 {{name}}", en: "Deleted device {{name}}"},
	KeyOperlogDeviceStart:         {zh: "启动设备 {{name}}", en: "Started device {{name}}"},
	KeyOperlogDeviceStop:          {zh: "停止设备 {{name}}", en: "Stopped device {{name}}"},
	KeyOperlogDeviceEnable:        {zh: "启用设备 {{name}}", en: "Enabled device {{name}}"},
	KeyOperlogDeviceDisable:       {zh: "禁用设备 {{name}}", en: "Disabled device {{name}}"},
	KeyOperlogTemplateCreate:      {zh: "创建模板 {{name}}", en: "Created template {{name}}"},
	KeyOperlogTemplateUpdate:      {zh: "修改模板 {{name}}", en: "Updated template {{name}}"},
	KeyOperlogTemplateDelete:      {zh: "删除模板 {{name}}", en: "Deleted template {{name}}"},
	KeyOperlogUserCreate:          {zh: "创建用户 {{username}}", en: "Created user {{username}}"},
	KeyOperlogUserUpdate:          {zh: "修改用户 {{username}}", en: "Updated user {{username}}"},
	KeyOperlogUserDelete:          {zh: "删除用户 {{username}}", en: "Deleted user {{username}}"},
	KeyOperlogSystemUpdate:        {zh: "修改系统配置 {{name}}", en: "Updated system config {{name}}"},
	KeyOperlogDeleted:             {zh: "操作日志已清除", en: "Operation logs cleaned"},
	KeyOperlogQueryFailed:         {zh: "查询操作日志失败", en: "Failed to query operation logs"},
	KeyOperlogDeleteFailed:        {zh: "删除操作日志失败", en: "Failed to delete operation logs"},
	KeyOperlogLinkageActionCreate: {zh: "创建联动动作组 {{name}}", en: "Created action group {{name}}"},
	KeyOperlogLinkageActionUpdate: {zh: "修改联动动作组 {{name}}", en: "Updated action group {{name}}"},
	KeyOperlogLinkageActionDelete: {zh: "删除联动动作组 {{name}}", en: "Deleted action group {{name}}"},
	KeyOperlogLinkageRuleCreate:   {zh: "创建联动规则 {{name}}", en: "Created linkage rule {{name}}"},
	KeyOperlogLinkageRuleUpdate:   {zh: "修改联动规则 {{name}}", en: "Updated linkage rule {{name}}"},
	KeyOperlogLinkageRuleDelete:   {zh: "删除联动规则 {{name}}", en: "Deleted linkage rule {{name}}"},

	"maintenance.restore_point_create_failed": {zh: "生成配置还原点失败，已中止导入：{{error}}", en: "Failed to create a configuration restore point; import aborted: {{error}}"},
	"maintenance.restore_point_invalid_name":  {zh: "还原点名称无效", en: "Invalid restore point name"},
	"maintenance.restore_point_list_failed":   {zh: "查询配置还原点失败", en: "Failed to list configuration restore points"},
	"maintenance.config_restore_failed":       {zh: "配置还原失败：{{error}}", en: "Failed to restore configuration: {{error}}"},
}

var aliasToKey = map[string]string{
	"success":     KeyCommonSuccess,
	"参数错误":        KeyCommonInvalidParams,
	"无效的模板ID":     KeyTemplateInvalidID,
	"模板不存在":       KeyTemplateNotFound,
	"无效的指标ID":     KeyMetricInvalidID,
	"指标不存在":       KeyMetricNotFound,
	"查询指标失败":      KeyMetricQueryFailed,
	"创建成功":        KeyCommonCreateSuccess,
	"更新成功":        KeyCommonUpdateSuccess,
	"删除成功":        KeyCommonDeleteSuccess,
	"保存成功":        KeyCommonSaveSuccess,
	"添加成功":        KeyCommonCreateSuccess,
	"校验通过":        KeyMetricExprCheckOK,
	"未授权":         KeyCommonUnauthorized,
	"未提供Token":    KeyCommonTokenMissing,
	"Token无效或已过期": KeyCommonTokenInvalid,
	"权限不足":        KeyCommonForbidden,
	"删除用户失败":      KeyCommonInternalError,
}

func Translate(locale, key string, args map[string]interface{}) string {
	pack, ok := dictionary[key]
	if !ok {
		return key
	}
	text := pack.zh
	if IsEnglishLocale(locale) {
		text = pack.en
	}
	return applyArgs(text, args)
}

func Localize(locale, keyOrMessage string, args map[string]interface{}) string {
	key := strings.TrimSpace(keyOrMessage)
	if key == "" {
		return ""
	}
	if translated, ok := translateKnown(locale, key, args); ok {
		return translated
	}
	if translated, ok := localizeWithPrefix(locale, key); ok {
		return translated
	}
	return applyArgs(key, args)
}

func translateKnown(locale, key string, args map[string]interface{}) (string, bool) {
	if _, ok := dictionary[key]; ok {
		return Translate(locale, key, args), true
	}
	if canonical, ok := aliasToKey[key]; ok {
		return Translate(locale, canonical, args), true
	}
	return "", false
}

func localizeWithPrefix(locale, key string) (string, bool) {
	separators := []string{"：", ":"}
	for _, sep := range separators {
		idx := strings.Index(key, sep)
		if idx <= 0 {
			continue
		}
		prefix := strings.TrimSpace(key[:idx])
		suffix := strings.TrimSpace(key[idx+len(sep):])
		translatedPrefix, ok := translateKnown(locale, prefix, nil)
		if !ok {
			continue
		}
		if suffix == "" {
			return translatedPrefix, true
		}
		if sep == ":" {
			return translatedPrefix + ": " + suffix, true
		}
		return translatedPrefix + "：" + suffix, true
	}
	return "", false
}

func applyArgs(text string, args map[string]interface{}) string {
	if len(args) == 0 {
		return text
	}
	out := text
	for k, v := range args {
		out = strings.ReplaceAll(out, "{{"+k+"}}", fmt.Sprint(v))
	}
	return out
}
