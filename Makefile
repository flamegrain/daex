# daeX — 本地编辑，测试主机上运行/调试/发布
#
# 本地 (macOS) 常用:
#   make dev-setup   首次: 同步代码 + 安装依赖 + 安装并启动 dev systemd 服务
#   make watch       持续同步本地改动到测试机 (air / Vite 自动热重载)
#   make sync        手动同步一次
#   make logs        跟踪 dev 服务日志
#   make status      查看 dev / dae 服务状态
#   make deploy      构建并发布生产版 (systemd: daex, :9091)
#   make ssh         SSH 到测试机项目目录
#
# 可在本地新建 .vm_host 文件配置目标主机 (已加入 gitignore)，或通过环境变量 VM=root@<IP> 指定
VM          ?= $(shell [ -f .vm_host ] && cat .vm_host || echo "root@192.168.1.100")
REMOTE      ?= /opt/daex
SSH_OPTS    ?= -o ControlMaster=auto -o ControlPath=/tmp/daex-ssh-%C -o ControlPersist=10m
SSH         := ssh $(SSH_OPTS) $(VM)
RSYNC_FLAGS ?= -rlptz
VM_ENV      := . /etc/profile.d/daex-dev.sh 2>/dev/null || . /etc/profile.d/dae-webui-dev.sh 2>/dev/null;

RSYNC_EXCLUDES := --exclude .git/ --exclude node_modules/ --exclude backend/tmp/ --exclude bin/ \
                  --exclude backend/internal/webui/dist/ --exclude '*.log' --exclude .DS_Store

.PHONY: sync watch dev-setup dev-restart dev-stop logs status deploy ssh pull-lock \
        install uninstall package doctor check vm-deps vm-dev-install vm-build vm-deploy

# ---------------- local ----------------
sync:
	@rsync $(RSYNC_FLAGS) --delete $(RSYNC_EXCLUDES) -e "ssh $(SSH_OPTS)" ./ $(VM):$(REMOTE)/

watch:
	@./scripts/watch.sh

dev-setup: sync
	$(SSH) '$(VM_ENV) cd $(REMOTE) && make vm-deps vm-dev-install'

dev-restart: sync
	$(SSH) 'systemctl restart daex-dev-api daex-dev-web'

dev-stop:
	$(SSH) 'systemctl stop daex-dev-api daex-dev-web'

logs:
	$(SSH) -t 'journalctl -f -n 50 -u daex-dev-api -u daex-dev-web'

status:
	@$(SSH) 'systemctl --no-pager -n 0 status daex-dev-api daex-dev-web daex dae 2>/dev/null | grep -E "●|Active:"'

# 把 VM 上生成的 lockfile 拉回本地 (pnpm add 之后执行)
pull-lock:
	rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/frontend/package.json $(VM):$(REMOTE)/frontend/pnpm-lock.yaml frontend/
	rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/backend/go.mod backend/
	-rsync -az -e "ssh $(SSH_OPTS)" $(VM):$(REMOTE)/backend/go.sum backend/ 2>/dev/null

deploy: sync
	$(SSH) '$(VM_ENV) cd $(REMOTE) && make vm-build vm-deploy'

ssh:
	ssh -t $(VM) 'cd $(REMOTE) && exec $$SHELL -l'

# ---------------- local install / package ----------------
install:
	@sudo ./install.sh install

uninstall:
	@sudo ./install.sh uninstall

package:
	@./scripts/build-release.sh

doctor:
	@./scripts/doctor.sh

check:
	@./scripts/doctor.sh

# ---------------- on VM ----------------
vm-deps:
	cd frontend && pnpm install
	cd backend && go mod download

vm-dev-install:
	install -m 644 deploy/daex-dev-api.service deploy/daex-dev-web.service /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable --now daex-dev-api daex-dev-web
	systemctl restart daex-dev-api daex-dev-web

vm-build:
	cd frontend && pnpm install --frozen-lockfile && pnpm build
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$$(date +%Y%m%d-%H%M)" -o ../bin/daex .

vm-deploy:
	install -m 755 bin/daex /usr/local/bin/daex
	rm -f /usr/local/bin/dae-webui 2>/dev/null || true
	install -m 644 deploy/daex.service /etc/systemd/system/
	systemctl daemon-reload
	systemctl enable daex
	systemctl restart daex
	@sleep 1; systemctl --no-pager -n 5 status daex | head -12
