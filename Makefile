# dst-waystone 本地开发与验证
#
# 默认 `make dev` 在 127.0.0.1:8788 启动管理端，
# 用 `.tmp-data/` 模拟容器内的 `/data`，仓库根目录作为静态资源根。
#
# 本机一般没有 supervisord，因此 `/api/restart` 和 `/api/server/status`
# 在 dev 模式下会失败；MOD 配置生成、cluster 文件落盘、token 写入
# 等流程仍可正常验证。
#
# 常用：
#   make dev              # 启动开发服务（前台运行）
#   make build            # go build
#   make check            # go vet + go build + bash -n entrypoint
#   make image            # docker build dst-waystone:local
#   make compose-up       # docker compose up -d（在 docker/ 目录）
#   make seed-key         # 生成一个随机 admin key 供本地 .env 使用

PORT      ?= 8788
LISTEN    ?= 127.0.0.1
DEV_DATA  ?= $(CURDIR)/.tmp-data
DEV_ROOT  ?= $(CURDIR)
DEV_KEY_FILE ?= $(CURDIR)/.dev-key
ADMIN_KEY ?= $(shell cat $(DEV_KEY_FILE) 2>/dev/null)
IMAGE     ?= dst-waystone:local

GO ?= go

.DEFAULT_GOAL := help

.PHONY: help
help:
	@awk 'BEGIN { FS = ":.*##" } /^[a-zA-Z_-]+:.*?##/ { printf "  \033[1m%-14s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: dev
dev: $(DEV_KEY_FILE) ## 启动管理端开发服务（自动读取 .dev-key）
	@mkdir -p $(DEV_DATA)
	@echo "[dev] root=$(DEV_ROOT) dst-dir=$(DEV_DATA) listen=$(LISTEN):$(PORT)"
	@echo "[dev] admin key 已从 .dev-key 读取（make seed-key 可重新生成）"
	@echo "[dev] 打开 http://$(LISTEN):$(PORT)/，本地无 supervisord，重启/状态接口会报错。"
	DST_ADMIN_KEY=$(ADMIN_KEY) $(GO) run ./cmd/mod-manager \
		-root $(DEV_ROOT) \
		-dst-dir $(DEV_DATA) \
		-listen $(LISTEN) \
		-port $(PORT)

$(DEV_KEY_FILE):
	@umask 077 && openssl rand -base64 32 | tr -d '\n' > $@
	@echo "[dev] 已生成 $@（mode 0600，已加入 .gitignore）"

.PHONY: stop
stop: ## 停止本地 make dev 启动的 mod-manager 进程
	@pids="$$(lsof -t -nP -iTCP:$(PORT) -sTCP:LISTEN 2>/dev/null) $$(pgrep -f 'go run ./cmd/mod-manager' 2>/dev/null) $$(pgrep -f 'cmd/mod-manager/mod-manager' 2>/dev/null) $$(pgrep -x mod-manager 2>/dev/null)"; \
	pids="$$(echo $$pids | tr ' ' '\n' | sort -u | tr '\n' ' ')"; \
	if [ -z "$$(echo $$pids | tr -d ' ')" ]; then \
		echo "[stop] 没有发现监听 :$(PORT) 或运行中的 mod-manager 进程"; \
	else \
		echo "[stop] 终止进程：$$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		still="$$(lsof -t -nP -iTCP:$(PORT) -sTCP:LISTEN 2>/dev/null)"; \
		if [ -n "$$still" ]; then echo "[stop] 强制 kill -9 $$still"; kill -9 $$still 2>/dev/null || true; fi; \
		echo "[stop] 已停止"; \
	fi

.PHONY: build
build: ## 编译 mod-manager 二进制到 ./bin/mod-manager
	@mkdir -p bin
	$(GO) build -o bin/mod-manager ./cmd/mod-manager

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: test
test: ## go test
	$(GO) test ./...

.PHONY: check
check: vet ## 快速验收：go vet + go build + bash 语法检查
	$(GO) build -o /tmp/dst-mod-manager-check ./cmd/mod-manager
	bash -n docker/entrypoint.sh
	@echo "check ok"

.PHONY: clean-dev
clean-dev: ## 删除本地 .tmp-data 与 .dev-key
	rm -rf $(DEV_DATA) $(DEV_KEY_FILE)

.PHONY: clean
clean: clean-dev ## 清理构建产物与 .tmp-data
	rm -rf bin/

.PHONY: image
image: ## docker build dst-waystone:local
	docker build -f docker/Dockerfile -t $(IMAGE) .

.PHONY: compose-up
compose-up: ## 在 docker/ 目录 docker compose up -d
	cd docker && docker compose up -d

.PHONY: compose-down
compose-down: ## 在 docker/ 目录 docker compose down
	cd docker && docker compose down

.PHONY: compose-logs
compose-logs: ## docker compose logs -f --tail=200
	cd docker && docker compose logs -f --tail=200

.PHONY: seed-key
seed-key: ## 重新生成 .dev-key（覆盖现有，mode 0600）
	@umask 077 && openssl rand -base64 32 | tr -d '\n' > $(DEV_KEY_FILE)
	@echo "[dev] 已写入 $(DEV_KEY_FILE)："
	@cat $(DEV_KEY_FILE); echo
