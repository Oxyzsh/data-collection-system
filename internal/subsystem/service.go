package subsystem

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	collectormodule "data-collection-system/internal/collector"
	devicemodule "data-collection-system/internal/device"
	templatebiz "data-collection-system/internal/template/domain"
	templateruntime "data-collection-system/internal/template/runtime"
	"data-collection-system/internal/valueobj"
)

var (
	// ErrInvalidPointStr pointStr 格式非法。
	ErrInvalidPointStr = errors.New("pointStr 格式无效，应为 <deviceId>.<pointId>，例如 1.2")
	// ErrPointNotFound 测点不存在。
	ErrPointNotFound = errors.New("测点不存在")
	// ErrNoData 测点暂无实时数据（设备未运行或尚未采集）。
	ErrNoData = errors.New("测点暂无实时数据（设备可能未运行）")
)

// PointRef 是 pointStr 的解析结果。
type PointRef struct {
	DeviceID int
	PointID  int
}

// PointStr 生成对外的测点标识："<deviceId>.<pointId>"。
func PointStr(deviceID, pointID int) string {
	return fmt.Sprintf("%d.%d", deviceID, pointID)
}

// ParsePointStr 解析测点标识（如 "1.2"）。
func ParsePointStr(pointStr string) (PointRef, error) {
	parts := strings.Split(strings.TrimSpace(pointStr), ".")
	if len(parts) != 2 {
		return PointRef{}, ErrInvalidPointStr
	}
	deviceID, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	pointID, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || deviceID < 0 || pointID < 0 {
		return PointRef{}, ErrInvalidPointStr
	}
	return PointRef{DeviceID: deviceID, PointID: pointID}, nil
}

// ProtocolDataType 把系统数据类型映射为协议 dataType（指南 §5.2）。
func ProtocolDataType(dataType string) int {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case valueobj.DataTypeFLOAT64:
		return DataTypeFloat
	case valueobj.DataTypeSTRING:
		return DataTypeString
	default: // INT64 / BOOL
		return DataTypeInt
	}
}

// ProtocolRWType 把系统读写属性映射为协议 rwType（指南 §5.3）。
func ProtocolRWType(access string) int {
	if strings.Contains(strings.ToLower(strings.TrimSpace(access)), "w") {
		return RWReadWrite
	}
	return RWReadOnly
}

// BuildDeviceTree 用当前设备与模板测点构造设备树（指南 §3.2.2）：
// 每个设备是一个根节点，其测点（含采集器内置的在线状态点 0）是叶子节点。
func BuildDeviceTree() ([]DeviceTreeNode, error) {
	devices, err := devicemodule.GetQueryService().GetDevices()
	if err != nil {
		return nil, err
	}

	tree := make([]DeviceTreeNode, 0, len(devices))
	for _, device := range devices {
		rootID := deviceNodeID(device.ID)
		root := DeviceTreeNode{
			ID:       rootID,
			Name:     device.Name,
			ParentID: "",
			NodeType: NodeTypeRoot,
			IsLeaf:   false,
		}

		// 模板测点即设备树叶子；模板通常已内置 point_id=0 的在线状态点，
		// 若缺失才补一个（采集器会周期写入该点的在线状态）。
		hasOnlinePoint := false
		if cfg, err := templateruntime.GetTemplateConfig(device.TemplateID); err == nil {
			for _, point := range cfg.Points {
				if point.PointID == 0 {
					hasOnlinePoint = true
				}
				root.Children = append(root.Children, buildLeaf(device.ID, point))
			}
		}
		if !hasOnlinePoint {
			online := buildLeaf(device.ID, templatebiz.DevicePoint{
				PointID:  0,
				Name:     "设备在线状态",
				DataType: valueobj.DataTypeINT64,
				Access:   "r",
				Enabled:  1,
			})
			root.Children = append([]*DeviceTreeNode{online}, root.Children...)
		}
		tree = append(tree, root)
	}
	return tree, nil
}

// Realtime 返回指定测点的当前值（指南 §3.2.3）。
func Realtime(pointStr string) (*RealtimeData, error) {
	ref, err := ParsePointStr(pointStr)
	if err != nil {
		return nil, err
	}
	if _, err := LookupPoint(ref); err != nil {
		return nil, err
	}

	values, running := collectormodule.GetService().GetPointValues(ref.DeviceID, []int{ref.PointID})
	if !running {
		return nil, ErrNoData
	}
	item, ok := values[ref.PointID]
	if !ok || !item.Found {
		return nil, ErrNoData
	}

	ts := item.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	point, _ := LookupPoint(ref)
	return &RealtimeData{
		PointID:   PointStr(ref.DeviceID, ref.PointID),
		Timestamp: ts.UnixMilli(),
		Value:     valueToAny(item.Value, point.DataType),
	}, nil
}

// SetValue 写入指定测点（指南 §3.2.4），写入链路复用系统内的测点写值能力。
func SetValue(pointStr string, value any) error {
	ref, err := ParsePointStr(pointStr)
	if err != nil {
		return err
	}
	point, err := LookupPoint(ref)
	if err != nil {
		return err
	}
	converted, err := toCollectedValue(value, point.DataType)
	if err != nil {
		return err
	}
	return collectormodule.GetService().WritePoint(ref.DeviceID, ref.PointID, converted)
}

// LookupPoint 校验测点存在并返回其模板定义。
func LookupPoint(ref PointRef) (templatebiz.DevicePoint, error) {
	devices, err := devicemodule.GetQueryService().GetDevices()
	if err != nil {
		return templatebiz.DevicePoint{}, err
	}
	var templateID int
	found := false
	for _, device := range devices {
		if device.ID == ref.DeviceID {
			templateID = device.TemplateID
			found = true
			break
		}
	}
	if !found {
		return templatebiz.DevicePoint{}, fmt.Errorf("%w: 设备 %d 不存在", ErrPointNotFound, ref.DeviceID)
	}

	// 在线状态点由采集器内置维护，不在模板里。
	if ref.PointID == 0 {
		return templatebiz.DevicePoint{
			PointID:  0,
			Name:     "设备在线状态",
			DataType: valueobj.DataTypeINT64,
			Access:   "r",
			Enabled:  1,
		}, nil
	}

	cfg, err := templateruntime.GetTemplateConfig(templateID)
	if err != nil {
		return templatebiz.DevicePoint{}, fmt.Errorf("%w: 模板 %d 不存在", ErrPointNotFound, templateID)
	}
	for _, point := range cfg.Points {
		if point.PointID == ref.PointID {
			return point, nil
		}
	}
	return templatebiz.DevicePoint{}, fmt.Errorf("%w: %s", ErrPointNotFound, PointStr(ref.DeviceID, ref.PointID))
}

func deviceNodeID(deviceID int) string {
	return fmt.Sprintf("device:%d", deviceID)
}

func buildLeaf(deviceID int, point templatebiz.DevicePoint) *DeviceTreeNode {
	return &DeviceTreeNode{
		ID:       fmt.Sprintf("point:%s", PointStr(deviceID, point.PointID)),
		Name:     point.Name,
		ParentID: deviceNodeID(deviceID),
		NodeType: NodeTypeLeaf,
		IsLeaf:   true,
		PointID:  PointStr(deviceID, point.PointID),
		Unit:     point.Unit,
		DataType: ProtocolDataType(point.DataType),
		RWType:   ProtocolRWType(point.Access),
	}
}

func valueToAny(value valueobj.CollectedValue, dataType string) any {
	switch value.Kind {
	case valueobj.KindText:
		return value.Text
	case valueobj.KindBool:
		return value.Bool
	default:
		if strings.EqualFold(dataType, valueobj.DataTypeINT64) {
			return int64(math.Round(value.Number))
		}
		return value.Number
	}
}

func toCollectedValue(value any, dataType string) (valueobj.CollectedValue, error) {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case valueobj.DataTypeBOOL:
		if b, ok := value.(bool); ok {
			return valueobj.NewBoolValue(b), nil
		}
		f, err := toFloat(value)
		if err != nil {
			return valueobj.CollectedValue{}, err
		}
		return valueobj.NewBoolValue(f != 0), nil
	case valueobj.DataTypeSTRING:
		if value == nil {
			return valueobj.NewTextValue(""), nil
		}
		if s, ok := value.(string); ok {
			return valueobj.NewTextValue(s), nil
		}
		return valueobj.NewTextValue(fmt.Sprint(value)), nil
	default:
		f, err := toFloat(value)
		if err != nil {
			return valueobj.CollectedValue{}, err
		}
		if strings.EqualFold(dataType, valueobj.DataTypeINT64) {
			f = math.Round(f)
		}
		return valueobj.NewNumberValue(f), nil
	}
}

func toFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("值 %q 不是有效数字", v)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("值类型 %T 不受支持", value)
	}
}
