<p align="center">
  <img src="frontend/public/logo-128.png" width="96" height="96" alt="daeX Logo" />
</p>

<h1 align="center">daeX 控制台</h1>

<p align="center">
  <b>新一代 Linux eBPF 透明代理控制台与现代化可视化网关面板</b>
  <br />
  <i>轻量级 · 高性能 · 单二进制嵌入零依赖 · 现代美观 · 100% 完整中英双语</i>
</p>

<p align="center">
  <a href="README.md"><b>简体中文</b></a> | <a href="README_EN.md"><b>English</b></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%20%7C%20Vue3%20%7C%20TypeScript-blue.svg" alt="Language" />
  <img src="https://img.shields.io/badge/Core-Linux%20eBPF-orange.svg" alt="Kernel" />
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License" />
</p>

---

## 📸 界面预览 (UI Screenshots)

### 1. 实时网络流量大盘与节点测速 (Dashboard)
毫秒级上/下行动态速率折线、内核工作状态、实时出站连通性模拟测试（阿里、Google、GitHub、YouTube、OpenAI）与就绪代理节点概览。
<p align="center">
  <img src="docs/screenshots/dashboard.png" width="100%" alt="daeX Dashboard" />
</p>

---

### 2. 4 步沉浸式新手快速设置向导 (Quick Setup Wizard)
专为非专业用户设计的快速向导：智能探测物理网卡与部署场景、订阅与多格式节点批量导入、预设分流联动、直观三色（快速/慢速/无法连接）连通性诊断并自动归档方案。
<p align="center">
  <img src="docs/screenshots/quick-setup.png" width="100%" alt="daeX Quick Setup" />
</p>

---

### 3. 多方案配置管理与语法校验 (Configuration Management)
支持多套 `.dae` 方案独立归档、一键设为生效、实时语法合法性校验、错误模块标红与源码精准高亮、配置文件快速导入/导出备份。
<p align="center">
  <img src="docs/screenshots/config.png" width="100%" alt="daeX Configuration Management" />
</p>

---

### 4. 可视化分流规则引擎与预设库 (Routing Rules)
遵循 dae 官方「首条命中即终止 (First-Match-Wins)」原则，提供自由拖拽排序、健康遮蔽检测、国内直连保护与官方高频模板库（基础直连、境外代理、广告阻断等）。
<p align="center">
  <img src="docs/screenshots/routing.png" width="100%" alt="daeX Routing Rules" />
</p>

---

### 5. 节点与出站策略组管理 (Nodes & Policies)
支持 VLESS, VMess, Trojan, Shadowsocks, Hysteria2 等协议节点解析、多分组策略调度（min_moving_avg、min_avg10、fallback、random）及订阅源自动拉取。
<p align="center">
  <img src="docs/screenshots/nodes.png" width="100%" alt="daeX Nodes and Policies" />
</p>

---

### 6. 系统服务管理与一键自检 (Service & Diagnostics)
无缝对接 systemd 守护进程，提供运行指标（PID、内存、CPU、运行时长）、启动/停止/重启/重载全生命周期操作，内置一键系统健康诊断报告。
<p align="center">
  <img src="docs/screenshots/service.png" width="100%" alt="daeX Service Management" />
</p>

---

## 🌟 核心功能特性

- 🚀 **单二进制嵌入架构**：基于 Go 标准库后端 + 嵌入式 Vue 3 单页前端，无外置 Python、PHP 或 Nginx 依赖，单文件极速部署，开箱即用。
- 🪄 **快速设置向导**：面向非专业用户的 4 步极简向导，智能推荐旁路由/单机网卡参数，支持机场订阅与节点链接一键解析导入，提供醒目三色测速与一键应用归档。
- 📊 **实时网络流量大盘**：毫秒级上/下行实时速率流、动态速率折线图表，以及一键连通性快速探测面板（阿里云、Google、GitHub、YouTube、OpenAI）。
- 🔀 **可视化条件分流规则引擎**：严格遵循 dae 官方决策原则，内置结构化条件构造器、局域网私网直连与兜底规则保护，支持鼠标自由拖拽排序与规则遮蔽检测。
- 📚 **官方预设模板库与语法指南**：内置基础直连、广告阻断、境外加速、AI 工具等高频预设模板，支持一键填入表单及在线同步最新模板。
- 🌐 **节点与出站策略组管理**：支持多种协议节点识别与批量解析、多策略分组调度（min_moving_avg、min_avg10 等）及外部订阅源聚合。
- 📜 **全功能日志管理与调试**：系统分流日志实时捕获、级别筛选高亮（INFO/WARN/ERROR/DEBUG）、关键字快速检索、独立下拉快照备份管理及日志文件导出。
- ⚙️ **系统服务管理控制面板**：无缝对接 systemd，提供 dae 核心与 WebUI 控制台的实时状态感知、运行指标、生命周期操作及最近 journalctl 日志集成。
- 🩺 **全方位系统自检诊断 (Doctor)**：一键排查 Linux 内核版本、eBPF 挂载、配置语法合法性与网卡路由健康。
- 🌓 **双主题与多语言**：支持深色/浅色界面平滑切换，100% 完整中英文语言环境无缝切换。

---

## ⚡ 自动化安装与部署 (一键安装)

### 1. 在线一键安装 (推荐)

在 Linux 服务器（需 root 权限）上执行以下单行命令，脚本将自动识别硬件架构并配置 systemd 守护进程：

```bash
curl -fsSL https://raw.githubusercontent.com/flamegrain/daex/main/install.sh | bash
```

> **提示**：如果中国大陆地区访问 GitHub 较慢，可通过 Git 镜像或克隆源码执行。

### 2. 自定义参数安装

支持在安装命令后追加自定义参数（例如自定义端口、指定 dae 配置文件路径）：

```bash
# 自定义监听端口 9091 并指定 dae 配置文件
curl -fsSL https://raw.githubusercontent.com/flamegrain/daex/main/install.sh | bash -s -- --port 9091 --config /usr/local/etc/dae/config.dae
```

### 3. 从源码目录就地安装

如果你已将本项目代码克隆至服务器，可直接执行根目录下的自动化脚本：

```bash
git clone https://github.com/flamegrain/daex.git /opt/daex
cd /opt/daex

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
| `restart` | 重启 daex 服务 |
| `status` | 查看当前 daex 服务运行状态与资源占用 |
| `logs` | 实时跟踪查看 daex 的 systemd 服务日志 |
| `stop` | 停止 daex 服务 |
| `start` | 启动 daex 服务 |
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

daeX 默认注册为标准的 systemd 服务：`daex.service`（同时创建 `/usr/local/bin/dae-webui` 兼容软链接）。

```bash
# 查看服务运行状态
systemctl status daex

# 重启 WebUI 服务
systemctl restart daex

# 停止 WebUI 服务
systemctl stop daex

# 查看实时运行日志 (Follow)
journalctl -u daex -f

# 设置开机自启
systemctl enable daex
```

---

## ⚙️ 启动参数与环境变量

若需手动启动或在脚本中运行，可使用以下命令行选项或等价的环境变量：

| 启动标志 (Flag) | 环境变量 (ENV) | 默认值 | 作用说明 |
|---|---|---|---|
| `-listen` | `DAE_WEBUI_LISTEN` | `0.0.0.0:9091` | HTTP Web 监听地址与端口 |
| `-dae-config` | `DAE_CONFIG` | *(必填)* | 关联的 dae 配置文件路径 |
| `-dae-bin` | `DAE_BIN` | `/usr/local/bin/dae` | dae 核心程序执行路径 |
| `-dae-service` | `DAE_SERVICE` | `dae` | 底层由 systemd 托管的 dae 服务名称 |
| `-backup-dir` | `DAE_BACKUP_DIR` | `<config_dir>/backups` | 配置文件历史备份与快照目录 |
| `-allow-system-config` | `DAE_ALLOW_SYSTEM_CONFIG` | `false` | 允许直接加载 `/etc/dae/` 或 `/usr/local/etc/dae/` 生产目录中的配置文件 |

---

## 🩺 系统自检与运行环境诊断 (Doctor)

dae 基于 Linux 内核 eBPF 技术，对操作系统内核版本、BPF 文件系统挂载及路由规则格式有特定要求。项目提供命令行与 Web 控制台双模自检诊断：

### 1. 命令行自检
```bash
sudo ./install.sh doctor
```

自检项目涵盖：
- ✅ **Linux 内核与架构**（要求 Linux 5.8+，推荐 5.15+）
- ✅ **eBPF 文件系统**（验证 `/sys/fs/bpf` 挂载状态）
- ✅ **systemd 服务管理器**（运行模式与服务就绪度）
- ✅ **dae 核心主程序**（二进制执行权限与版本）
- ✅ **dae 配置文件语法**（自动调用 `dae validate -c` 检测合法性）
- ✅ **路由规则库资源**（检查 `geosite.dat` 与 `geoip.dat` 完整性）
- ✅ **daeX 服务运行状态**（systemd 活跃度与 HTTP 端口监听）
- ✅ **配置安全权限**（验证敏感配置文件权限是否符合 `0600`）

---

## 💻 开发者指南 (从源码编译)

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

# 2. 编译后端并生成二进制 (输出到 bin/daex)
cd ../backend
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(date +%Y%m%d-%H%M)" -o ../bin/daex .
```

---

## 🔒 数据安全与最佳实践

1. **配置文件权限保护**：所有生成的 dae 配置文件与备份均自动受 `0600` 严格文件权限保护，避免非特权用户读取代理凭证。
2. **反向代理与 HTTPS**：推荐配合 Nginx / Caddy 等反向代理工具配置 SSL/TLS 证书，开启 HTTPS 加密访问面板。
3. **安全审计日志**：重要配置修改及版本切换均在备份目录下保留历史快照，支持一键回滚。

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 协议开源。
