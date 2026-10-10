package collector

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

const (
	simulatorHalfCycleSamples = 10
	simulatorFullCycleSamples = simulatorHalfCycleSamples * 2

	simulatorTemperatureLow  = 32.8
	simulatorTemperatureHigh = 66.8
	simulatorHumidityLow     = 58.2
	simulatorHumidityHigh    = 83.2

	simulatorTemperatureJitter = 0.4
	simulatorHumidityJitter    = 0.6
)

type SimulatorCollector struct {
	mu          sync.Mutex
	sampleIndex int
	rng         *rand.Rand
	writtenVals map[int]float64
}

func init() {
	RegisterCollector("simulator", func(params map[string]string) Collector {
		return newSimulatorCollector()
	})
}

func newSimulatorCollector() *SimulatorCollector {
	return &SimulatorCollector{
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		writtenVals: make(map[int]float64),
	}
}

func (c *SimulatorCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	result := make(map[int]valueobj.CollectedValue)

	logger.Debugf("[SimulatorCollector] 设备ID: %d, 测点数: %d\n", deviceID, len(points))

	temperature, humidity := c.nextEnvironmentValues()

	for _, point := range points {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		var value float64
		c.mu.Lock()
		wVal, hasWritten := c.writtenVals[point.PointID]
		c.mu.Unlock()

		if hasWritten {
			value = wVal
		} else {
			switch point.PointID {
			case 1:
				value = temperature
			case 2:
				value = humidity
			default:
				value = c.syntheticValueForOtherPoint(point.PointID)
			}
		}
		result[point.PointID] = valueobj.NewNumberValue(value)
	}

	return result, nil
}

// Write 实现：缓存写入的控制量，让下次 Collect 时能够读回。
func (c *SimulatorCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	var fval float64
	switch v := rawValue.(type) {
	case float64:
		fval = v
	case float32:
		fval = float64(v)
	case int64:
		fval = float64(v)
	case int32:
		fval = float64(v)
	case int:
		fval = float64(v)
	case bool:
		if v {
			fval = 1
		} else {
			fval = 0
		}
	default:
		return fmt.Errorf("模拟采集器不支持的数据类型: %T", rawValue)
	}

	c.mu.Lock()
	if c.writtenVals == nil {
		c.writtenVals = make(map[int]float64)
	}
	c.writtenVals[point.PointID] = fval
	c.mu.Unlock()

	logger.Debugf("[SimulatorCollector] 设备ID: %d, 写入测点 %d = %f\n", deviceID, point.PointID, fval)
	return nil
}

func (c *SimulatorCollector) Stop() {
	logger.Infof("[SimulatorCollector] 停止采集")
}

func (c *SimulatorCollector) nextEnvironmentValues() (float64, float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rng == nil {
		c.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	sample := c.sampleIndex
	c.sampleIndex++

	temperature := c.waveValue(sample, simulatorTemperatureLow, simulatorTemperatureHigh, simulatorTemperatureJitter, 0)
	humidity := c.waveValue(sample, simulatorHumidityLow, simulatorHumidityHigh, simulatorHumidityJitter, 2)
	return temperature, humidity
}

func (c *SimulatorCollector) waveValue(sample int, low, high, jitterAmplitude float64, phaseOffset int) float64 {
	position := (sample + phaseOffset + simulatorFullCycleSamples) % simulatorFullCycleSamples

	var ratio float64
	if position < simulatorHalfCycleSamples {
		ratio = float64(position) / float64(simulatorHalfCycleSamples-1)
	} else {
		ratio = float64(position-simulatorHalfCycleSamples) / float64(simulatorHalfCycleSamples-1)
		ratio = 1 - ratio
	}

	// 使用 cosine easing，让趋势更接近真实环境中的缓慢升降。
	eased := 0.5 - 0.5*math.Cos(math.Pi*ratio)
	base := low + (high-low)*eased
	jitter := (c.rng.Float64()*2 - 1) * jitterAmplitude
	value := base + jitter

	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func (c *SimulatorCollector) syntheticValueForOtherPoint(pointID int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rng == nil {
		c.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return 40 + float64((pointID%7)*5) + c.rng.Float64()*6
}
