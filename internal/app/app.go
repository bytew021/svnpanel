package app

import (
	"fmt"

	"svnpanel/internal/app/center"
	"svnpanel/internal/cfg"
	"svnpanel/internal/service"
	"svnpanel/pkg/httpx"
)

func Initialize(confPath string) (func(), error) {
	config, err := cfg.Load(confPath)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureRuntimeDirs(); err != nil {
		return nil, err
	}
	svc := service.New(config)
	if err := svc.Validate(); err != nil {
		return nil, fmt.Errorf("启动校验失败: %w", err)
	}
	engine := httpx.NewEngine(*config)
	center.Config(engine, svc)
	server, err := httpx.Start(*config, engine)
	if err != nil {
		return nil, err
	}
	return func() {
		_ = server.Stop()
	}, nil
}
