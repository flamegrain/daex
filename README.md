# dae WebUI

<p align="center">
  <b>新一代 Linux eBPF 透明代理控制台与可视化网关面板</b>
  <br />
  <i>轻量级 · 高性能 · 单二进制零依赖 · 现代美观 · 100% 完整中英双语</i>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%20%7C%20Vue3%20%7C%20TypeScript-blue.svg" alt="Language" />
  <img src="https://img.shields.io/badge/Core-Linux%20eBPF-orange.svg" alt="Kernel" />
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License" />
</p>

---

## 🌟 核心功能特性

- 🚀 **单二进制嵌入架构**：基于 Go 标准库后端 + 嵌入式 Vue 3 单页前端，无外置 Python、PHP 或 Nginx 依赖，单文件极速部署，开箱即用。
- 📊 **实时网络流量大盘**：毫秒级上/下行实时速率流、动态速率折线图表，以及一键连通性快速探测面板（阿里云、Google、GitHub、YouTube、OpenAI）。
- 🔀 **可视化条件分流规则引擎**：严格遵循 dae 官方「首条命中即终止 (First-Match-Wins)」决策原则，内置结构化条件构造器、局域网私网直连与兜底规则保护，支持鼠标自由拖拽排序。
- 📚 **官方预设模板库与语法指南**：内置基础直连、广告阻断、境外加速、AI 工具等高频预设模板，支持一键填入表单及在线同步最新模板。
- 🌐 **节点与出站策略组管理**：支持多种协议节点识别与批量解析、多策略分组调度（min_moving_avg、min_avg10 等）及外部订阅源聚合。
- 📜 **全功能日志管理与调试**：系统分流日志实时捕获、级别筛选高亮（INFO/WARN/ERROR/DEBUG）、关键字快速检索、独立下拉快照备份管理及日志文件导出。
- ⚙️ **系统服务管理控制面板**：无缝对接 systemd，提供 dae 核心与 WebUI 控制台的实时状态感知、运行指标（PID、内存、CPU、运行时长）、生命周期操作（启动、停止、重启、热重载、开机自启）及最近 journalctl 日志集成。
- 🔄 **系统自动更新与规则库同步**：直接对接 GitHub 官方发行版本，可视化展示更新日志并支持一键升级 dae 主程序及 geosite/geoip 规则库。
- 🌓 **双主题与多语言**：支持深色/浅色界面平滑切换，100% 完整中英文语言环境无缝切换。

---

## ⚡ 自动化安装与部署 (一键安装)

### 1. 在线一键安装 (推荐)

在 Linux 服务器（需 root 权限）上执行以下单行命令，脚本将自动识别硬件架构并配置 systemd 守护进程：

```bash
curl -fsSL https://raw.githubusercontent.com/daeuniverse/dae-webui/main/install.sh | bash
```

> **提示**：如果中国大陆地区访问 GitHub 较慢，可通过 Git 镜像或克隆源码执行。

### 2. 自定义参数安装

支持在安装命令后追加自定义参数（例如自定义端口、指定 dae 配置文件路径）：

```bash
# 自定义监听端口 9091 并指定 dae 配置文件
curl -fsSL https://raw.githubusercontent.com/daeuniverse/dae-webui/main/install.sh | bash -s -- --port 9091 --config /usr/local/etc/dae/config.dae
```

### 3. 从源码目录就地安装

如果你已将本项目代码克隆至服务器，可直接执行根目录下的自动化脚本：

```bash
git clone https://github.com/daeuniverse/dae-webui.git /opt/dae-webui
cd /opt/dae-webui

# 执行自动化安装与服务注册
sudo ./install.sh install
```

---

## 🛠️ 安装脚本参数与子命令

`install.sh` 提供了完备的日常运维子命令，方便管理员快速维护服务：

```bash
sudo ./install.sh [命令] [选项]
```

### 常用子命令

| 命令 | 说明 |
|---|---|
| `install` | 检查环境、安装程序并注册 systemd 服务（默认） |
| `doctor` / `check` | 系统运行环境与健康自检（内核、eBPF、配置语法、服务等） |
| `update` | 检查并拉取最新版本更新程序 |
| `restart` | 重启 dae-webui 服务 |
| `status` | 查看当前 dae-webui 服务运行状态与资源占用 |
| `logs` | 实时跟踪查看 dae-webui 的 systemd 服务日志 |
| `stop` | 停止 dae-webui 服务 |
| `start` | 启动 dae-webui 服务 |
| `uninstall` | 安全卸载服务并清理二进制文件（默认保留用户配置） |

### 选项说明

| 参数选项 | 默认值 | 说明 |
|---|---|---|
| `--port <PORT>` | `9091` | WebUI 网页控制台监听端口 |
| `--host <HOST>` | `0.0.0.0` | WebUI 监听地址 |
| `--config <PATH>` | *自动探测* | dae 配置文件路径（如未指定则自动检测已有系统配置） |
| `--bin <PATH>` | `/usr/local/bin/dae` | dae 核心程序绝对路径 |
| `--service <NAME>` | `dae` | 底层管理的 dae systemd 服务名称 |
| `--no-start` | `false` | 安装完成后不立即拉起服务 |
| `-y, --yes` | `false` | 非交互式静默安装 |

---

## 🔑 访问与初始登录

安装成功后，终端将输出欢迎信息面板：

- **控制台地址**：`http://<服务器IP>:9091`
- **默认用户名**：`admin`
- **默认登录密码**：`admin123`

> ⚠️ **安全建议**：首次登录后，请立即前往左侧导航栏 **「系统设置」** 页面修改默认登录密码。

---

## 🖥️ systemd 系统服务常用管理

dae-webui 默认注册为标准的 systemd 服务：`dae-webui.service`。

```bash
# 查看服务运行状态
systemctl status dae-webui

# 重启 WebUI 服务
systemctl restart dae-webui

# 停止 WebUI 服务
systemctl stop dae-webui

# 查看实时运行日志 (Follow)
journalctl -u dae-webui -f

# 设置开机自启
systemctl enable dae-webui
```

---

## ⚙️ 启动参数与环境变量

若需手动启动或在容器/脚本中运行，可使用以下命令行选项或等价的环境变量：

| 启动标志 (Flag) | 环境变量 (ENV) | 默认值 | 作用说明 |
|---|---|---|---|
| `-listen` | `DAE_WEBUI_LISTEN` | `0.0.0.0:9091` | HTTP Web 监听地址与端口 |
| `-dae-config` | `DAE_CONFIG` | *(必填)* | 关联的 dae 配置文件路径 |
| `-dae-bin` | `DAE_BIN` | `/usr/local/bin/dae` | dae 核心程序执行路径 |
| `-dae-service` | `DAE_SERVICE` | `dae` | 底层由 systemd 托管的 dae 服务名称 |
| `-backup-dir` | `DAE_BACKUP_DIR` | `<config_dir>/backups` | 配置文件历史备份与快照目录 |
| `-allow-system-config` | - | `false` | 允许直接加载 `/usr/local/etc/dae/` 生产目录中的配置文件 |

---

## 🩺 系统自检与运行环境诊断 (Doctor / Self-Check)

dae 基于 Linux 内核 eBPF 技术，对操作系统内核版本、BPF 文件系统挂载及路由规则格式有特定要求。项目提供命令行与 Web 控制台双模自检诊断：

### 1. 命令行自检
```bash
# 方式一：使用安装管理脚本
sudo ./install.sh doctor

# 方式二：直接执行自检脚本
sudo ./scripts/doctor.sh

# 方式三：开发者环境
make doctor
```

自检项目涵盖：
- ✅ **Linux 内核与架构**（要求 Linux 5.8+，推荐 5.15+）
- ✅ **eBPF 文件系统**（验证 `/sys/fs/bpf` 挂载状态）
- ✅ **systemd 服务管理器**（运行模式与服务就绪度）
- ✅ **dae 核心主程序**（二进制执行权限与版本）
- ✅ **dae 配置文件语法**（自动调用 `dae validate -c` 检测合法性）
- ✅ **路由规则库资源**（检查 `geosite.dat` 与 `geoip.dat` 完整性）
- ✅ **dae-webui 服务运行状态**（systemd 活跃度与 HTTP 端口监听）
- ✅ **默认出口网络路由**（检查默认网关与对外接口）
- ✅ **配置安全权限**（验证敏感配置文件权限是否符合 `0600`）

### 2. WebUI 网页一键自检
在管理后台左侧导航栏 **「服务管理」** 页面，点击顶部 **「🩺 系统自检」** 按钮，即可实时触发多维度健康诊断并弹出可视化体检报告，支持一键复制体检报告用于运维排障。

---

## 🗑️ 安全卸载与清理 (Uninstall)

项目提供独立的卸载脚本 `uninstall.sh`，同时也支持通过 `./install.sh uninstall` 执行：

### 1. 标准卸载（推荐，保留配置与数据）
停止并注销 systemd 服务，移除可执行二进制文件与系统服务文件，**完整保留** `/etc/dae-webui/` 配置文件与日志数据：
```bash
sudo ./uninstall.sh
# 或
sudo ./install.sh uninstall
```

### 2. 彻底清除卸载（Purge）
如需连同所有配置文件、规则草稿、备份快照和日志文件一并清除：
```bash
sudo ./uninstall.sh --purge
# 或静默非交互式彻底清理：
sudo ./uninstall.sh -y --purge
```

---

## 💻 开发者指南 (从源码编译与打包)

### 前置环境需求
- **Go** >= 1.22 (推荐 1.24+)
- **Node.js** >= 18.x
- **pnpm** >= 8.x

### 1. 本地编译
```bash
# 1. 编译前端
cd frontend
pnpm install --frozen-lockfile
pnpm build

# 2. 编译后端并生成二进制 (输出到 bin/dae-webui)
cd ../backend
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(date +%Y%m%d-%H%M)" -o ../bin/dae-webui .
```

### 2. 跨平台打包发布
执行项目提供的自动化打包脚本，可自动生成 Linux x86_64 与 aarch64 的发布包及校验和：
```bash
./scripts/build-release.sh [版本号]
```
构建产物将保存在 `release/` 目录中：
- `dae-webui-linux-amd64.tar.gz`
- `dae-webui-linux-arm64.tar.gz`
- `sha256sums.txt`

---

## 🔒 数据安全与生产最佳实践

1. **配置文件权限保护**：所有生成的 dae 配置文件与备份均自动受 `0600` 严格文件权限保护，避免非特权用户读取代理凭证。
2. **反向代理与 HTTPS**：推荐配合 Nginx / Caddy 等反向代理工具配置 SSL/TLS 证书，开启 HTTPS 加密访问面板。
3. **安全审计日志**：重要配置修改及版本切换均在 `/var/log/dae-webui/` 目录下保留历史版本，支持在管理后台一键回滚。

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 协议开源。
EOF
