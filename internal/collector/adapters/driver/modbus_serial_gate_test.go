package collector

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeGateClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeGateClock() *fakeGateClock {
	return &fakeGateClock{now: time.Unix(0, 0)}
}

func (c *fakeGateClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeGateClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func TestSerialGateBlocksConcurrentSameSerial(t *testing.T) {
	clock := newFakeGateClock()
	mgr := &requestGateManager{
		gates: map[string]*requestGate{},
		now:   clock.Now,
		wait: func(context.Context, time.Duration) error {
			return nil
		},
	}

	release1, err := mgr.acquire(context.Background(), "/dev/ttyUSB0", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release1()

	done := make(chan struct{})
	go func() {
		release2, err := mgr.acquire(context.Background(), "/dev/ttyUSB0", 30*time.Millisecond)
		if err != nil {
			t.Errorf("second acquire failed: %v", err)
			close(done)
			return
		}
		defer release2()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("second acquire should block while first request holds the serial gate")
	case <-time.After(40 * time.Millisecond):
	}

	release1()
	release1 = func() {}

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second acquire did not complete after the first request released the gate")
	}
}

func TestSerialGateWaitsForInterval(t *testing.T) {
	clock := newFakeGateClock()
	var waits []time.Duration
	mgr := &requestGateManager{
		gates: map[string]*requestGate{},
		now:   clock.Now,
		wait: func(_ context.Context, d time.Duration) error {
			waits = append(waits, d)
			clock.Advance(d)
			return nil
		},
	}

	release1, err := mgr.acquire(context.Background(), "/dev/ttyUSB1", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	release1()

	release2, err := mgr.acquire(context.Background(), "/dev/ttyUSB1", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	defer release2()

	if len(waits) != 1 {
		t.Fatalf("expected one wait, got %d", len(waits))
	}
	if waits[0] != 30*time.Millisecond {
		t.Fatalf("expected wait 30ms, got %v", waits[0])
	}
}

func TestSerialGateDifferentSerialsIndependent(t *testing.T) {
	clock := newFakeGateClock()
	mgr := &requestGateManager{
		gates: map[string]*requestGate{},
		now:   clock.Now,
		wait:  func(context.Context, time.Duration) error { return nil },
	}

	release1, err := mgr.acquire(context.Background(), "/dev/ttyUSB2", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release1()

	done := make(chan struct{})
	go func() {
		release2, err := mgr.acquire(context.Background(), "/dev/ttyUSB3", 30*time.Millisecond)
		if err != nil {
			t.Errorf("second acquire failed: %v", err)
			close(done)
			return
		}
		defer release2()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
		t.Fatal("different serial paths should not block each other")
	}
}
