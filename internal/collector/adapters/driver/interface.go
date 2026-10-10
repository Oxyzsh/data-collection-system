package collector

import (
	"context"
	"sync"

	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

type Collector interface {
	// Collect 执行一次设备采集。
	// points 和 params 都是上层提前准备好的快照，collector 不直接查数据库。
	// 这让协议采集器可以专注于“如何和设备通信、如何解码返回值”，
	// 而把配置装配交给更上游的 infrastructure/runtime 层。
	Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error)
	Stop()

	// Write 向下行写入单个测点的原始值。
	// rawValue 是已按逆公式转换后的**原始类型值**（如 int16/uint16/float32/[]byte 等），
	// driver 需要以协议对应的写功能码/Set 操作写入设备。
	// 仅当模板 Access 含写权限的测点可达此处。
	Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error
}

// CollectorFactory 根据采集方法创建具体 collector。
// 这相当于把 “modbus/snmp/simulator -> 具体实现类型” 的选择逻辑集中在一个入口。
type CollectorFactory interface {
	CreateCollector(collectMethod string, params map[string]string) Collector
}

// CollectorConstructor 是注册到工厂里的构造函数签名。
// Go 里函数本身就是一等值，所以这里直接用“函数类型”做注册表项，而不是额外定义工厂对象。
type CollectorConstructor func(params map[string]string) Collector

// DefaultCollectorFactory 是默认工厂实现。
// 具体协议采集器在各自文件的 init() 中完成注册。
type DefaultCollectorFactory struct{}

func (f *DefaultCollectorFactory) CreateCollector(collectMethod string, params map[string]string) Collector {
	if ctor := getCollectorConstructor(collectMethod); ctor != nil {
		return ctor(params)
	}
	// 保底回落到模拟采集器，避免未知类型导致运行中断。
	if ctor := getCollectorConstructor("simulator"); ctor != nil {
		return ctor(params)
	}
	return &SimulatorCollector{}
}

var defaultFactory = &DefaultCollectorFactory{}
var (
	collectorRegistryMu sync.RWMutex
	collectorRegistry   = map[string]CollectorConstructor{}
)

// RegisterCollector 注册一种采集方法对应的构造函数。
// 注册表是进程内全局状态，因此需要互斥锁保护并发读写。
func RegisterCollector(collectMethod string, constructor CollectorConstructor) {
	if collectMethod == "" || constructor == nil {
		return
	}
	collectorRegistryMu.Lock()
	defer collectorRegistryMu.Unlock()
	collectorRegistry[collectMethod] = constructor
}

// getCollectorConstructor 查询已注册的采集构造函数。
func getCollectorConstructor(collectMethod string) CollectorConstructor {
	collectorRegistryMu.RLock()
	defer collectorRegistryMu.RUnlock()
	return collectorRegistry[collectMethod]
}

// GetCollectorFactory 获取采集器工厂实例。
// 上层通常只和这个工厂接口交互，不直接 new 某个协议采集器。
func GetCollectorFactory() CollectorFactory {
	return defaultFactory
}
