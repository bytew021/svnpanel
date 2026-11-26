package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"svnpanel/internal/app"
	"svnpanel/pkg/version"
)

var (
	showVersion = flag.Bool("version", false, "显示版本信息")
	confPath    = flag.String("conf", "etc/config.toml", "配置文件路径")
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	flag.Parse()
	version.Version = Version
	version.BuildTime = BuildTime

	if *showVersion {
		fmt.Printf("svnpanel %s (%s)\n", Version, BuildTime)
		return
	}
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "不支持位置参数")
		os.Exit(2)
	}

	cleanup, err := app.Initialize(*confPath)
	if err != nil {
		slog.Error("服务初始化失败", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signals
	slog.Info("收到退出信号", "signal", sig.String(), "time", time.Now().Format(time.RFC3339))
}
