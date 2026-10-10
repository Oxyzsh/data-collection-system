package bootstrap

import "sync"

// 进程停止编排。
//
// main 包不能被子包 import，而版本升级 / 回滚 / 恢复出厂三条流程都需要
// 「先停掉全部后台任务与数据库，再落盘、再原地重启」。按项目既有的
// 「闭包注册回调打破依赖方向」惯例（如 sharedws.SetSelectorSnapshotResolver），
// 由 main 注册、由 system 模块触发。
var (
	shutdownMu    sync.Mutex
	shutdownHooks []func()
	shutdownRan   bool
)

// RegisterShutdownHook 注册一个进程停止钩子，按注册顺序执行。
func RegisterShutdownHook(fn func()) {
	if fn == nil {
		return
	}
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	if shutdownRan {
		// 已经跑过停止编排（进程正在退出或已 exec 重启），再注册也不会执行。
		return
	}
	shutdownHooks = append(shutdownHooks, fn)
}

// RunShutdownHooks 依次执行全部钩子，幂等：重复调用只生效一次。
//
// 幂等是防御性的：正常退出路径上 main 的收尾会调用一次，而 exec 成功后
// 旧进程的 defer 永远不会执行（进程映像被替换），实际不会重复触发。
func RunShutdownHooks() {
	shutdownMu.Lock()
	if shutdownRan {
		shutdownMu.Unlock()
		return
	}
	shutdownRan = true
	hooks := shutdownHooks
	shutdownMu.Unlock()

	for _, fn := range hooks {
		fn()
	}
}
