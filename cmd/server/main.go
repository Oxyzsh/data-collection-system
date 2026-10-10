package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"data-collection-system/internal/bootstrap"
	collectormodule "data-collection-system/internal/collector"
	"data-collection-system/internal/database"
	devicemodule "data-collection-system/internal/device"
	historymodule "data-collection-system/internal/history"
	httprouter "data-collection-system/internal/interfaces/http"
	"data-collection-system/internal/interfaces/httpx/locale"
	runtimemodule "data-collection-system/internal/runtime"
	runtimeinfra "data-collection-system/internal/runtime/infrastructure"
	"data-collection-system/internal/shared/buildinfo"
	"data-collection-system/internal/shared/logger"
	templatemodule "data-collection-system/internal/template"
)

// webUI 是前端构建产物（frontend/ -> cmd/server/dist，由 build.sh 复制）。
// 使用 all: 前缀保证目录里只有 .keep 占位文件时也能通过编译。
//
//go:embed all:dist
var webUI embed.FS

// main 是「数据采集进程」的启动入口，只保留采集主链路：
//
//	设备模板导入 → 按模板添加设备 → 采集运行 → 运行态 / 历史数据落库
//
// 已裁剪：认证、告警、操作日志、系统运维、视频、门禁、组态、联动、北向、维护等模块。
func main() {
	// SIGINT/SIGTERM 走正常返回路径，保证 defer 里的停机上钩子执行。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	settings := bootstrap.LoadSettings()
	logger.Setup(settings.Log.Level)
	if err := bootstrap.EnsureRuntimeDirectories(settings); err != nil {
		logger.Fatal("创建运行目录失败: %v", err)
	}
	version := buildinfo.Load()

	initPersistence()
	defer bootstrap.RunShutdownHooks()

	initModules()
	startBackgroundJobs(settings)

	// 停机上钩子：注册顺序即执行顺序。
	bootstrap.RegisterShutdownHook(func() {
		historyWriter := runtimeinfra.GetHourlyHistoryWriter()
		historyWriter.Stop()
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := historyWriter.Flush(flushCtx); err != nil {
			logger.Errorf("[History] 关机落盘存在失败项: %v\n", err)
		}
	})
	bootstrap.RegisterShutdownHook(database.CloseDB)

	r := newHTTPServer(version)

	srv := &http.Server{
		Addr:    ":" + settings.ServerPort,
		Handler: r,
	}
	logger.Infof("数据采集系统已启动: %s %s", version, "http://localhost:"+settings.ServerPort)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("服务启动失败: %v", err)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warnf("HTTP 服务优雅关闭超时: %v", err)
	}
}

// initPersistence 准备配置库 / 运行时库 / 历史库三个 SQLite 连接。
func initPersistence() {
	if err := database.InitDB(); err != nil {
		logger.Fatal("数据库初始化失败: %v", err)
	}
}

// initModules 按依赖顺序初始化采集主链路各模块的表结构与缓存。
func initModules() {
	if err := runtimemodule.InitOnStartup(); err != nil {
		logger.Fatal("初始化运行时表失败: %v", err)
	}
	if err := historymodule.InitOnStartup(); err != nil {
		logger.Fatal("初始化历史表失败: %v", err)
	}
	if err := devicemodule.InitOnStartup(); err != nil {
		logger.Fatal("初始化设备模块失败: %v", err)
	}
	if err := templatemodule.InitOnStartup(); err != nil {
		logger.Fatal("初始化设备模板失败: %v", err)
	}
	// selector 实时值查询服务（设备 selector-values 接口使用）。
	devicemodule.GetSelectorRealtimeService()
}

// startBackgroundJobs 启动与采集主链路相关的后台任务。
func startBackgroundJobs(settings bootstrap.Settings) {
	// 冷启动恢复：把配置为启用/运行中的设备重新拉起采集线程。
	collectormodule.GetService().RestoreEnabledDevices()

	// 历史落盘调度：整点前窗口按设备错峰把内存缓冲写入历史库。
	historyWriter := runtimeinfra.GetHourlyHistoryWriter()
	if err := historyWriter.LoadDailyExtremum(context.Background(), database.HistoryDB); err != nil {
		logger.Warnf("[History] 装载当日极值失败（继续运行）: %v\n", err)
	}
	historyWriter.SetExtremumFlushInterval(settings.Storage.ExtremumFlushInterval)
	historyWriter.Start(time.Minute, settings.Storage.FlushWindowMinutes)
}

// newHTTPServer 装配 HTTP 服务：中间件 + 采集业务路由 + 健康检查。
func newHTTPServer(version string) *gin.Engine {
	// 默认 release 模式（减少路由表等调试输出），显式设置 GIN_MODE=debug 时保留调试行为。
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(accessLogMiddleware())
	r.Use(corsMiddleware())
	r.Use(locale.Middleware())
	r.Use(locale.ResponseMessageLocalizer())

	httprouter.RegisterRoutes(r)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	})

	registerStaticRoutes(r)

	return r
}

// registerStaticRoutes 挂载内嵌前端页面：
// - 命中 dist 中的静态文件则直接返回；
// - 其余非 /api 路径回退到 index.html（单页应用路由）。
func registerStaticRoutes(r *gin.Engine) {
	dist, err := fs.Sub(webUI, "dist")
	if err != nil {
		logger.Fatal("无法加载前端资源: %v", err)
	}
	fileServer := http.FileServer(http.FS(dist))

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if strings.HasPrefix(reqPath, "/api/") || reqPath == "/healthz" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
			return
		}

		clean := strings.TrimPrefix(path.Clean(reqPath), "/")
		isAsset := strings.HasPrefix(clean, "assets/")
		if clean != "" && clean != "." && !strings.HasPrefix(clean, "..") {
			if f, err := dist.Open(clean); err == nil {
				_ = f.Close()
				// Vite 产物文件名带内容 hash：可长缓存；其余静态文件每次校验。
				if isAsset {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					c.Header("Cache-Control", "no-cache")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// 缺失的前端资源必须明确 404：绝不能回退成 index.html，
		// 否则浏览器会把 HTML 当 JS/CSS 解析，页面卡死在加载中。
		if isAsset {
			c.Status(http.StatusNotFound)
			return
		}

		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				[]byte("<h3>前端未构建：请在项目根目录执行 ./build.sh frontend 后重新编译后端</h3>"))
			return
		}
		// 入口 HTML 禁止缓存：保证刷新后一定拿到引用最新资源的版本。
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

// corsMiddleware 允许跨域，便于独立部署时被其它系统/前端调用。
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept-Language")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// accessLogMiddleware 以 DEBUG 级记录访问日志（默认 info 级不输出）。
func accessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Debugf("[GIN] %d | %s | %s | %s %s",
			c.Writer.Status(), time.Since(start).Round(time.Millisecond),
			c.ClientIP(), c.Request.Method, c.Request.URL.Path)
	}
}
