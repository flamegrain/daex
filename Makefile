# dae-webui — 本地编辑，VM (192.168.3.10) 上运行/调试/发布
#
# 本地 (macOS) 常用:
#   make dev-setup   首次: 同步代码 + 安装依赖 + 安装并启动 dev systemd 服务
#   make watch       持续同步本地改动到 VM (air / Vite 自动热重载)
#   make sync        手动同步一次
#   make logs        跟踪 dev 服务日志
#   make status      查看 dev / dae 服务状态
#   make deploy      构建并发布生产版 (systemd: dae-webui, :9091)
#   make ssh         SSH 到 VM 项目目录
#
# vm-* 目标在 VM 上执行 (由上面的目标通过 ssh 调用)。

VM          ?= root@192.168.3.10
REMOTE      ?= /opt/dae-webui
SSH_OPTS    ?= -o ControlMaster=auto -o ControlPath=/tmp/dae-webui-ssh-%C -o ControlPersist=10m
SSH         := ssh $(SSH_OPTS) $(VM)
RSYNC_FLAGS ?= -rlptz
VM_ENV      := . /etc/profile.d/dae-webui-dev.sh;

RSYNC_EXCLUDES := --exclude .git/ --exclude node_modules/ --exclude backend/tmp/ --exclude bin/ \
                  --exclude backend/internal/webui/dist/ --exclude '*.log' --exclude .DS_Store

.PHONY: sync watch dev-setup dev-restart dev-stop logs status deploy ssh pull-lock \
        vm-deps vm-dev-install vm-build vm-deploy

# ---------------- local ----------------
sync:
	@rsync $(RSYNC_FLAGS) --delete $(RSYNC_EXCLUDES) -e "ssh $(SSH_OPTS)" ./ $(VM):$(REMOTE)/

watch:
	@./scripts/watch.sh

dev-setup: sync
	$(SSH) '$(VM_ENV) cd $(REMOTE) && make vm-deps vm-dev-install'

dev-restart: sync
	$(SSH) 'systemctl restart dae-webui-dev-api dae-webui-dev-web'

dev-stop:
	$(SSH) 'systemctl stop dae-webui-dev-api dae-webui-dev-web'

logs:
	$(SSH) -t 'journalctl -f -n 50 -u dae-webui-dev-api -u dae-webui-dev-web'

status:
	@$(SSH) 'systemctl --no-pager -n 0 status dae-webui-dev-api dae-webui-dev-web dae-webui dae 2>/dev/null | grep -E "●|Active:"'

# 把 VM 上生成的 lockfile 拉回本地 (pnpm add 之后执行)
pull-lock:
	rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/frontend/package.json $(VM):$(REMOTE)/frontend/pnpm-lock.yaml frontend/
	rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/backend/go.mod backend/
	-rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/backend/go.sum backend/ 2>/dev/null

deploy: sync
	$(SSH) '$(VM_ENV) cd $(REMOTE) && make vm-build vm-deploy'

ssh:
	ssh -t $(VM) 'cd $(REMOTE) && exec $$SHELL -l'

# ---------------- on VM ----------------
vm-deps:
	cd frontend && pnpm install
	cd backend && go mod download

vm-dev-install:
	install -m 644 deploy/dae-webui-dev-api.service deploy/dae-webui-dev-web.service /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable --now dae-webui-dev-api dae-webui-dev-web
	systemctl restart dae-webui-dev-api dae-webui-dev-web

vm-build:
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$$(date +%Y%m%d-%H%M)" -o ../bin/dae-webui .

vm-deploy:
	install -m 755 bin/dae-webui /usr/local/bin/dae-webui
	install -m 644 deploy/dae-webui.service /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable dae-webui
	systemctl restart dae-webui
	@sleep 1; systemctl --no-pager -n 5 status dae-webui | head -12
