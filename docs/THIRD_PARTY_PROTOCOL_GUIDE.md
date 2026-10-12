# Sub-System 三方协议对接指南

## 一、概述

本文档详细描述第三方系统与 Sub-System 模块对接的 HTTP 接口规范。`subsystem.go` 实现了与第三方系统的 HTTP 通信，包括设备树获取、实时数据采集、控制命令下发等功能。

---

## 二、对接架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Sub-System 服务                                 │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Device Manager                               │   │
│  │  (设备管理、状态监控、数据路由)                                  │   │
│  └──────────────────────────┬──────────────────────────────────────┘   │
│                             │                                          │
│                             ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Subsystem Instance                           │   │
│  │  ┌───────────────────────────────────────────────────────────┐ │   │
│  │  │  HttpClient: HTTP客户端                                   │ │   │
│  │  │  BaseURL: http://{device_ip}                             │ │   │
│  │  │  pointStrMap: map[modbusPointId] -> pointStr             │ │   │
│  │  │  stopChan: 停止信号通道                                   │ │   │
│  │  │  fetchInterval: 定时采集间隔(5s)                          │ │   │
│  │  └───────────────────────────────────────────────────────────┘ │   │
│  │                              │                                  │   │
│  │           ┌──────────────────┼──────────────────┐              │   │
│  │           │                  │                  │              │   │
│  │           ▼                  ▼                  ▼              │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐      │   │
│  │  │ Connect()     │  │ GetRealtime() │  │ SetValue()    │      │   │
│  │  │ /health       │  │ /realtime     │  │ /api/set      │      │   │
│  │  └───────────────┘  └───────────────┘  └───────────────┘      │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                              │
                              │ HTTP Requests
                              ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      第三方系统/设备                                   │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  API Endpoints:                                                │   │
│  │  ├─ GET  /health                    (健康检查)                 │   │
│  │  ├─ GET  /protocol/sub/system/api/v1/converter/deviceTree     │   │
│  │  │       (获取设备树)                                           │   │
│  │  ├─ GET  /protocol/sub/system/api/v1/converter/realtime       │   │
│  │  │       (获取实时数据)                                         │   │
│  │  └─ POST /api/set/{pointStr}        (设置测点值)               │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 核心组件说明

| 组件 | 职责 | 关键属性 |
|------|------|----------|
| **Subsystem** | 第三方系统接入实例 | HttpClient、BaseURL、pointStrMap |
| **DeviceManager** | 设备生命周期管理 | 设备加载、状态探测、数据转发 |
| **VirtualPointManager** | 虚拟测点管理 | 数据存储、告警规则、存储策略 |

---

## 三、三方接口规范

### 3.1 接口列表

| 接口 | HTTP方法 | 路径 | 功能 |
|------|----------|------|------|
| 健康检查 | GET | `/health` | 验证连接状态 |
| 获取设备树 | GET | `/protocol/sub/system/api/v1/converter/deviceTree` | 获取设备树结构 |
| 获取实时数据 | GET | `/protocol/sub/system/api/v1/converter/realtime` | 获取测点实时值 |
| 设置测点值 | POST | `/api/set/{pointStr}` | 下发控制命令 |

### 3.2 接口详细定义

#### 3.2.1 健康检查接口

**接口地址**: `GET /health`

**请求**: 无参数

**响应格式**:
```json
{
    "code": 0,
    "message": "success"
}
```

**功能**: 验证 Sub-System 与第三方系统的网络连接是否正常

#### 3.2.2 获取设备树接口

**接口地址**: `GET /protocol/sub/system/api/v1/converter/deviceTree`

**请求**: 无参数

**响应格式**:
```json
{
    "errorCode": 0,
    "errorMsg": "",
    "data": [
        {
            "id": "dev_001",
            "name": "设备名称",
            "parentId": "",
            "nodeType": 0,
            "children": [
                {
                    "id": "point_001",
                    "name": "测点名称",
                    "parentId": "dev_001",
                    "nodeType": 2,
                    "children": [],
                    "isLeaf": true,
                    "pointId": "device.temperature",
                    "unit": "°C",
                    "dataType": 1,
                    "rwType": 0
                }
            ],
            "isLeaf": false,
            "pointId": "",
            "unit": "",
            "dataType": 0,
            "rwType": 0
        }
    ]
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| errorCode | int | 错误码，0表示成功 |
| errorMsg | string | 错误信息 |
| data | array | 设备树节点列表 |
| data[].id | string | 节点唯一标识 |
| data[].name | string | 节点名称 |
| data[].parentId | string | 父节点ID（根节点为空） |
| data[].nodeType | int | 节点类型：0-根节点，1-中间节点，2-叶子节点 |
| data[].children | array | 子节点列表 |
| data[].isLeaf | bool | 是否为叶子节点（测点） |
| data[].pointId | string | 测点标识（仅叶子节点有效） |
| data[].unit | string | 测点单位 |
| data[].dataType | int | 数据类型：0-整数，1-浮点数，2-字符串，3-字节流 |
| data[].rwType | int | 读写类型：0-只读，1-读写 |

#### 3.2.3 获取实时数据接口

**接口地址**: `GET /protocol/sub/system/api/v1/converter/realtime`

**请求参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| pointStr | string | 是 | 测点标识 |

**响应格式**:
```json
{
    "errorCode": 0,
    "errorMsg": "",
    "data": {
        "pointId": "device.temperature",
        "timestamp": 1779176964968,
        "value": 25.5
    }
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| errorCode | int | 错误码，0表示成功 |
| errorMsg | string | 错误信息 |
| data.pointId | string | 测点标识 |
| data.timestamp | int64 | 数据时间戳（毫秒） |
| data.value | any | 测点值（整数/浮点数/字符串） |

#### 3.2.4 设置测点值接口

**接口地址**: `POST /api/set/{pointStr}`

**路径参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| pointStr | string | 测点标识 |

**请求体**:
```json
{
    "pointId": "device.temperature",
    "value": 28.0
}
```

**响应格式**:
```json
{
    "code": "0",
    "message": "success"
}
```

---

## 四、数据流转流程

### 4.1 设备树加载流程

```
Sub-System 启动
    ↓
创建 Subsystem 实例
    ↓
调用 Connect() → GET /health
    ↓
调用 loadDeviceTree() → GET /deviceTree
    ↓
解析 DeviceTreeResponse
    ↓
递归保存动态设备和测点到数据库
    ↓
启动定时任务 startPointValueFetcher()
    ↓
定时获取实时数据（每5秒）
```

### 4.2 实时数据获取流程

```
定时任务触发（每5秒）
    ↓
遍历 pointStrMap
    ↓
调用 GET /realtime?pointStr={pointStr}
    ↓
解析 RealtimeDataResponse
    ↓
调用 VirtualPointManager.SetValue()
    ↓
数据存入 Redis/时序数据库
    ↓
通过 RPC GetRealtimeData 获取数据
```

### 4.3 控制命令下发流程

```
RPC SetPointValue 请求
    ↓
DeviceManager.SetValue()
    ↓
Subsystem.SetValue()
    ↓
调用 POST /api/set/{pointStr}
    ↓
解析 SetValueResponse
    ↓
返回执行结果
```

---

## 五、数据结构定义

### 5.1 设备树节点

```go
type DeviceTreeNode struct {
    Id       string            `json:"id"`       // 节点ID
    Name     string            `json:"name"`     // 节点名称
    ParentId string            `json:"parentId"` // 父节点ID
    NodeType int               `json:"nodeType"` // 节点类型
    Children []*DeviceTreeNode `json:"children"` // 子节点
    IsLeaf   bool              `json:"isLeaf"`   // 是否叶子节点
    PointId  string            `json:"pointId"`  // 测点标识
    Unit     string            `json:"unit"`     // 单位
    DataType int               `json:"dataType"` // 数据类型
    RWType   int               `json:"rwType"`   // 读写类型
}
```

### 5.2 数据类型映射

| dataType值 | 说明 | Go类型 |
|------------|------|--------|
| 0 | 整数 | int |
| 1 | 浮点数 | float64 |
| 2 | 字符串 | string |
| 3 | 字节流 | []byte |

### 5.3 读写类型映射

| rwType值 | 说明 |
|----------|------|
| 0 | 只读 |
| 1 | 读写 |

---

## 六、对接示例

### 6.1 设备树响应示例

```json
{
    "errorCode": 0,
    "errorMsg": "",
    "data": [
        {
            "id": "main_device",
            "name": "主控制器",
            "parentId": "",
            "nodeType": 0,
            "children": [
                {
                    "id": "temperature_sensor",
                    "name": "温度传感器",
                    "parentId": "main_device",
                    "nodeType": 1,
                    "children": [
                        {
                            "id": "temp_room1",
                            "name": "房间1温度",
                            "parentId": "temperature_sensor",
                            "nodeType": 2,
                            "children": [],
                            "isLeaf": true,
                            "pointId": "main.temp.room1",
                            "unit": "°C",
                            "dataType": 1,
                            "rwType": 0
                        },
                        {
                            "id": "temp_room2",
                            "name": "房间2温度",
                            "parentId": "temperature_sensor",
                            "nodeType": 2,
                            "children": [],
                            "isLeaf": true,
                            "pointId": "main.temp.room2",
                            "unit": "°C",
                            "dataType": 1,
                            "rwType": 0
                        }
                    ],
                    "isLeaf": false,
                    "pointId": "",
                    "unit": "",
                    "dataType": 0,
                    "rwType": 0
                },
                {
                    "id": "humidity_sensor",
                    "name": "湿度传感器",
                    "parentId": "main_device",
                    "nodeType": 1,
                    "children": [
                        {
                            "id": "hum_room1",
                            "name": "房间1湿度",
                            "parentId": "humidity_sensor",
                            "nodeType": 2,
                            "children": [],
                            "isLeaf": true,
                            "pointId": "main.hum.room1",
                            "unit": "%",
                            "dataType": 1,
                            "rwType": 0
                        }
                    ],
                    "isLeaf": false,
                    "pointId": "",
                    "unit": "",
                    "dataType": 0,
                    "rwType": 0
                }
            ],
            "isLeaf": false,
            "pointId": "",
            "unit": "",
            "dataType": 0,
            "rwType": 0
        }
    ]
}
```

### 6.2 Python Flask 实现示例

```python
from flask import Flask, jsonify, request

app = Flask(__name__)

# 模拟设备树数据
device_tree = {
    "errorCode": 0,
    "errorMsg": "",
    "data": [
        {
            "id": "main_device",
            "name": "主控制器",
            "parentId": "",
            "nodeType": 0,
            "children": [
                {
                    "id": "temp_sensor",
                    "name": "温度传感器",
                    "parentId": "main_device",
                    "nodeType": 1,
                    "children": [
                        {
                            "id": "temp_001",
                            "name": "温度测点1",
                            "parentId": "temp_sensor",
                            "nodeType": 2,
                            "children": [],
                            "isLeaf": True,
                            "pointId": "device.temp.001",
                            "unit": "°C",
                            "dataType": 1,
                            "rwType": 0
                        }
                    ],
                    "isLeaf": False,
                    "pointId": "",
                    "unit": "",
                    "dataType": 0,
                    "rwType": 0
                }
            ],
            "isLeaf": False,
            "pointId": "",
            "unit": "",
            "dataType": 0,
            "rwType": 0
        }
    ]
}

# 模拟实时数据
realtime_data = {
    "device.temp.001": 25.5
}

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"code": 0, "message": "success"})

@app.route('/protocol/sub/system/api/v1/converter/deviceTree', methods=['GET'])
def get_device_tree():
    return jsonify(device_tree)

@app.route('/protocol/sub/system/api/v1/converter/realtime', methods=['GET'])
def get_realtime():
    point_str = request.args.get('pointStr')
    value = realtime_data.get(point_str, 0)
    
    return jsonify({
        "errorCode": 0,
        "errorMsg": "",
        "data": {
            "pointId": point_str,
            "timestamp": int(time.time() * 1000),
            "value": value
        }
    })

@app.route('/api/set/<point_str>', methods=['POST'])
def set_value(point_str):
    req = request.json
    value = req.get('value')
    
    realtime_data[point_str] = value
    
    return jsonify({"code": "0", "message": "success"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

---

## 七、错误码说明

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 非0 | 失败，具体错误信息在 errorMsg 或 message 字段 |

---

## 八、性能要求

### 8.1 响应时间

| 接口 | 要求 |
|------|------|
| /health | ≤ 100ms |
| /deviceTree | ≤ 500ms |
| /realtime | ≤ 200ms |
| /api/set | ≤ 500ms |


**文档版本**: v1.0  
**相关代码文件**: `sub-system/internal/device/subsystem/subsystem.go`
