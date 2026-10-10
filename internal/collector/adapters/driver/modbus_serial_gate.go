package collector

import (
	"context"
	"sync"
	"time"
)

type requestGate struct {
	mu         sync.Mutex
	lastFinish time.Time
}

// requestGateManager 按传输身份共享门禁状态。
// 这里不绑定到设备实例，是为了让同一串口或同一透传网关上的多个设备共享节流。
type requestGateManager struct {
	mu    sync.Mutex
	gates map[string]*requestGate
	now   func() time.Time
	wait  func(context.Context, time.Duration) error
}

func newRequestGateManager() *requestGateManager {
	return &requestGateManager{
		gates: make(map[string]*requestGate),
		now:   time.Now,
		wait:  waitForDuration,
	}
}

// acquire 在请求前占用门禁，并在释放时刷新最后完成时间。
// 这既保证同一 identity 上的请求串行，也保证最小请求间隔会跨设备生效。
func (m *requestGateManager) acquire(ctx context.Context, identity string, minInterval time.Duration) (func(), error) {
	if identity == "" || minInterval <= 0 {
		return func() {}, nil
	}

	gate := m.get(identity)
	gate.mu.Lock()

	if !gate.lastFinish.IsZero() {
		waitFor := minInterval - m.now().Sub(gate.lastFinish)
		if waitFor > 0 {
			if err := m.wait(ctx, waitFor); err != nil {
				gate.mu.Unlock()
				return nil, err
			}
		}
	}

	released := false
	return func() {
		if released {
			return
		}
		released = true
		gate.lastFinish = m.now()
		gate.mu.Unlock()
	}, nil
}

func (m *requestGateManager) get(identity string) *requestGate {
	m.mu.Lock()
	defer m.mu.Unlock()

	if gate, ok := m.gates[identity]; ok {
		return gate
	}

	gate := &requestGate{}
	m.gates[identity] = gate
	return gate
}

func waitForDuration(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var defaultModbusRequestGate = newRequestGateManager()
