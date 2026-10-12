// Package subsystem 按 docs/THIRD_PARTY_PROTOCOL_GUIDE.md 对外提供「三方子系统」接口。
//
// 本系统在对接关系中扮演「第三方系统/设备」一侧，向外部 Sub-System 暴露 4 个接口：
//
//	GET  /health                                            健康检查
//	GET  /protocol/sub/system/api/v1/converter/deviceTree   设备树
//	GET  /protocol/sub/system/api/v1/converter/realtime     测点实时值
//	POST /api/set/{pointStr}                                设置测点值
package subsystem

// 协议常量（对接指南 §3.2.2 / §5.2 / §5.3）。
const (
	NodeTypeRoot   = 0
	NodeTypeMiddle = 1
	NodeTypeLeaf   = 2

	DataTypeInt    = 0
	DataTypeFloat  = 1
	DataTypeString = 2
	DataTypeBytes  = 3

	RWReadOnly  = 0
	RWReadWrite = 1
)

// DeviceTreeNode 是设备树节点（指南 §5.1）。
type DeviceTreeNode struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	ParentID string            `json:"parentId"`
	NodeType int               `json:"nodeType"`
	Children []*DeviceTreeNode `json:"children"`
	IsLeaf   bool              `json:"isLeaf"`
	PointID  string            `json:"pointId"`
	Unit     string            `json:"unit"`
	DataType int               `json:"dataType"`
	RWType   int               `json:"rwType"`
}

// RealtimeData 是测点实时值（指南 §3.2.3）。
type RealtimeData struct {
	PointID   string `json:"pointId"`
	Timestamp int64  `json:"timestamp"`
	Value     any    `json:"value"`
}
