# dae WebUI

dae 的轻量可视化面板：配置管理、日志查看、节点状态、调试。
代码在本地编辑，**运行、调试、发布全部在 dae 虚拟机 (192.168.3.10) 上进行**。

## ⚠️ 数据安全规则

- 配置解析、节点展示等功能**只允许**使用 [`config.mock.dae`](config.mock.dae) 假数据开发和测试。
- 后端没有默认配置路径，必须显式传 `-dae-config`；指向 `/usr/local/etc/dae/` 时会拒绝启动（软链接也会被识别），除非显式加 `-allow-system-config`。
- `.gitignore` 屏蔽了除 mock 以外的 `*.dae` / 订阅文件。`scripts/git-hooks/pre-commit` 会拒绝任何含代理/订阅链接的提交（mock 文件除外）。
- 真实节点信息严禁提交、严禁推送到外网仓库。

## 架构

| | 开发 | 生产 |
|---|---|---|
| 前端 | Vite HMR `http://192.168.3.10:5173` | 嵌入 Go 二进制 |
| 后端 | air 热重载 `:8080`（Vite 代理 `/api`） | `/usr/local/bin/dae-webui` `:9091` |
| 进程 | `dae-webui-dev-api` / `dae-webui-dev-web` (systemd) | `dae-webui` (systemd) |

技术栈：Go 1.27（仅标准库）· Vue 3 + Vite 8 + TypeScript 6 · pnpm。
虚拟机工具链：`/usr/local/go`、`/usr/local/node`、`/root/go/bin/air`，PATH 在 `/etc/profile.d/dae-webui-dev.sh` 里设置。

## 日常开发（在本地 Mac 执行）

```bash
make watch        # 持续同步代码到 VM，air / Vite 自动热重载（建议先 brew install fswatch）
# 浏览器打开 http://192.168.3.10:5173

make logs         # 跟踪 dev 服务日志
make status       # dev / 生产 / dae 服务状态
make dev-restart  # 重启 dev 服务
make ssh          # 进入 VM 项目目录 /opt/dae-webui
make deploy       # 构建并发布生产版到 :9091
```

**加依赖**：在 VM 上执行 `cd /opt/dae-webui/frontend && pnpm add xxx`，然后在本地执行 `make pull-lock`，把 `package.json` 和 lockfile 拉回本地。不拉回的话，下次 sync 会把它们覆盖掉。

> 注意：`vue-tsc` 还不支持 TypeScript 7，`typescript` 暂时锁定在 `^6`。
