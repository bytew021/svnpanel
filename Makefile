VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILDTIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
CONF ?= etc/config.toml

.PHONY: help dev dev-windows frontend-install frontend-build build build-windows test lint fmt-check frontend-check verify clean init-dev

help:
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  %-18s %s\\n",$$1,$$2}'

init-dev: ## 初始化本地开发文件
	@mkdir -p data/repos data/backups logs/audit
	@touch etc/passwd etc/authz
	@chmod 600 etc/config.toml etc/passwd etc/authz 2>/dev/null || true

dev: ## 启动后端开发服务（另开终端运行 frontend dev）
	go run . -conf $(CONF)

dev-windows: ## 使用 Windows 本地路径启动开发服务
	go run . -conf etc/config.windows.toml

frontend-install: ## 安装前端依赖
	cd frontend && pnpm install --frozen-lockfile

frontend-build: ## 构建前端并复制到嵌入目录
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	rm -rf web/dist
	cp -a frontend/dist web/dist

build: frontend-build ## 构建包含前端资源的 Linux 二进制
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=embed \
		-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILDTIME)" \
		-o bin/svnpanel .

build-windows: frontend-build ## 构建包含前端资源的 Windows 二进制
	mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags=embed \
		-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILDTIME)" \
		-o bin/svnpanel.exe .

test: ## 运行后端测试
	go test ./... -cover

lint: ## 运行 go vet
	go vet ./...

fmt-check: ## 检查 Go 格式
	@out=$$(gofmt -l main.go internal pkg web); \
	if [ -n "$$out" ]; then echo "gofmt 未通过:"; echo "$$out"; exit 1; fi

frontend-check: ## 前端类型检查与构建
	cd frontend && pnpm typecheck && pnpm build

verify: fmt-check lint test frontend-check ## 本地提交门禁

clean: ## 清理构建产物
	rm -rf bin frontend/dist web/dist
