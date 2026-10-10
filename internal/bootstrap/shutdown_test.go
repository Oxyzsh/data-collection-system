package bootstrap

import (
	"testing"
)

func resetShutdownHooks(t *testing.T) {
	t.Helper()
	shutdownMu.Lock()
	shutdownHooks = nil
	shutdownRan = false
	shutdownMu.Unlock()
	t.Cleanup(func() {
		shutdownMu.Lock()
		shutdownHooks = nil
		shutdownRan = false
		shutdownMu.Unlock()
	})
}

func TestRunShutdownHooksExecutesInRegistrationOrder(t *testing.T) {
	resetShutdownHooks(t)
	var order []string
	RegisterShutdownHook(func() { order = append(order, "first") })
	RegisterShutdownHook(func() { order = append(order, "second") })

	RunShutdownHooks()

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("执行顺序 = %v, want [first second]", order)
	}
}

func TestRunShutdownHooksIsIdempotent(t *testing.T) {
	resetShutdownHooks(t)
	calls := 0
	RegisterShutdownHook(func() { calls++ })

	RunShutdownHooks()
	RunShutdownHooks()

	if calls != 1 {
		t.Fatalf("钩子执行次数 = %d, want 1（须幂等）", calls)
	}
}

func TestRegisterAfterRunDoesNotExecute(t *testing.T) {
	resetShutdownHooks(t)
	RunShutdownHooks()

	calls := 0
	RegisterShutdownHook(func() { calls++ })
	RunShutdownHooks()

	if calls != 0 {
		t.Fatalf("已执行过之后注册的钩子不应再执行，calls = %d", calls)
	}
}
