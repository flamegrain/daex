# daeX 项目上下文交接文档 (Context Handover Document)

**文档生成时间**：2026-10-22  
**项目名称**：daeX（基于 Linux eBPF 透明代理核心 `dae` 的新一代可视化管理控制台）  
**开源代码库**：[https://github.com/flamegrain/daex](https://github.com/flamegrain/daex)  
**当前版本**：`v1.0.0` (Git Tag & GitHub Releases 已同步)

---

## 一、系统架构与技术选型

### 1. 技术栈
- **前端 (Frontend)**：Vue 3 + TypeScript + Vite + Pinia/Composables + Lucide Icons + CodeMirror 6（配置文件代码高亮与语法校验）。
- **后端 (Backend)**：Go (标准库，零冗余重依赖，使用 `go:embed` 将前端构建产物嵌入单一二进制文件，启动内存仅约 10MB)。
- **打包分发**：支持跨平台编译（Linux `amd64` / `arm64`），一键发布为单独立可执行文件与 `.tar.gz` 压缩包。
- **运行守护**：基于 Linux Systemd 服务（`daex.service`），支持与底层的 `dae.service` 透明代理内核深度协同。

### 2. 核心功能全景
1. **网络管控大盘 (Dashboard)**：实时监测上行/下行速率、连接数、内存 CPU 消耗、dae 核心与控制台运行状态。
2. **可视化配置中心 (Config)**：严格遵循 dae 语法规范，支持 6 大配置模块的可视化表单与代码视图双向同步：
   - 1. 全局配置 (`global`)
   - 2. 节点订阅 (`subscription`)
   - 3. 手工节点 (`node`)
   - 4. DNS 智能分流 (`dns`)
   - 5. 策略分组 (`group`)
   - 6. 分流规则链 (`routing`)
3. **分流规则与健康诊断 (Routing)**：支持 30+ 种常用分流模板一键导入、规则拖拽排序、冲突与语法死锁静态分析。
4. **节点与策略监控 (Nodes)**：订阅节点解析、一键批量 Ping 测速、出站策略组候选节点监控与分流仿真追踪。
5. **服务诊断与更新 (Service & Update)**：Systemd 状态控制、日志实时回放、一键跨版本无缝升级。

---

## 二、测试服务器与部署环境

| 属性项 | 配置详情 |
| :--- | :--- |
| **服务器地址** | `192.168.3.10` (Debian/Ubuntu Linux) |
| **登录用户** | `root` (免密 SSH 密钥配置有效) |
| **项目工作目录** | `/opt/daex` *(注意：原 `/opt/dae-webui` 已彻底清空并移除)* |
| **编译环境变量** | `/etc/profile.d/daex-dev.sh` (包含 Go、Node、pnpm 路径) |
| **服务配置路径** | `/etc/systemd/system/daex.service` |
| **控制台访问端口** | `http://192.168.3.10:9091` |
| **dae 真实配置路径** | `/usr/local/etc/dae/config.dae` |
| **备份文件目录** | `/var/log/daex/backups` |
| **当前运行命令** | `/usr/local/bin/daex -listen 0.0.0.0:9091 -allow-system-config -backup-dir /var/log/daex/backups` |

---

## 三、近期重大演进与已解决问题

### 1. 项目全面重命名 (`dae-webui` -> `daex`)
- 二进制文件名统一变更为 `daex`，服务单元文件统一为 `daex.service`。
- 本地与远端部署路径由 `/opt/dae-webui` 迁移至 `/opt/daex`，清理所有旧路径残留。
- 一键安装脚本（`install.sh` / `uninstall.sh`）全面支持老用户平滑迁移与清理旧版本服务。
- GitHub Release 资产彻底清退旧版 `dae-webui-*` 文件，全新统一为 `daex-linux-amd64` / `daex-linux-arm64` 及其 `.tar.gz`。

### 2. 真实生产配置 vs 测试沙箱（Mock）动态判定机制
- **判定逻辑**：
  - 若系统检测到 `/usr/local/etc/dae/config.dae`、`/etc/dae/config.dae` 或 `/etc/daex/config.dae`，自动挂载真实生产配置，`isMock` 置为 `false`。
  - 界面左侧导航栏、状态指示灯显示 `真实配置`（绿色药丸标签）与 `实时生产数据`，隐藏所有 `mock` 徽标。
  - 只有在系统未安装任何真实配置时，才会回退挂载测试沙箱配置并显著标注。
- **隐私保护**：将 Mock 配置内的历史测试 IP 替换为 RFC 5737 官方合规保留文档 IP（`198.51.100.219`）。

### 3. 策略分组与节点/订阅联动及 `fixed(N)` 数字代码自动生成
- **多分组管理**：用户可在「配置管理 -> 策略分组」自由新增多个不同用途的分组（如 `proxy`、`streaming`、`japan`、`game`、`backup` 等）。
- **两步式向导**：
  - **第 1 步（选择来源）**：可选择「节点订阅联动 (`subtag`)」或「指定节点多选 (`name`)」，自动列出包含的所有候选节点及对应索引序号 `[0]`、`[1]`、`[2]`...
  - **第 2 步（选择模式）**：支持 `min_moving_avg`、`min_avg10`、`min`、`random` 与 **`fixed(N) 固定节点模式`**。
- **代码自动生成**：若选中 `fixed` 模式，直接点击候选节点，系统自动计算并填入数字序号生成 `policy: fixed(N)`，并在卡片上提供实时 `dae` 代码预览与绿标确认。
- **后端决策对齐**：完善 `backend/nodes_policy.go` 中的选路算法，当分组策略为 `fixed(N)` 时，精准锁定第 `N` 个匹配节点。

### 4. 分流规则右侧常驻滚动条
- 修复当分流规则列表（30~100+ 条）超出视窗高度时右侧无滚动条的问题。
- 将 `RoutingView.vue` 和 `ConfigView.vue` 的主滚动容器设置为 `flex: 1; min-height: 0; overflow-y: auto; scrollbar-gutter: stable;`，工具栏保持吸顶。
- 全局注入现代化的深色/浅色细滚动条样式（`::-webkit-scrollbar` 与 `scrollbar-width: thin`），彻底解决 macOS / Linux 默认隐藏滚动条的视觉隐患。

---

## 四、核心文件与工程目录映射

```text
daex/
├── backend/
│   ├── main.go               # 主入口、配置探测与挂载解析、RESTful 路由、静态资源嵌入
│   ├── service_manage.go     # Systemd 运行状态监控、进程与性能分析
│   └── nodes_policy.go       # 节点解析、fixed(N)策略路由算法、规则仿真追踪
├── frontend/
│   ├── src/
│   │   ├── views/
│   │   │   ├── DashboardView.vue  # 流量大盘与实时监控
│   │   │   ├── ConfigView.vue     # 可视化配置中心 (含策略分组联动与 fixed(N) 自动生成)
│   │   │   ├── RoutingView.vue    # 分流规则管理、拖拽排序、冲突诊断
│   │   │   └── NodesView.vue      # 节点延迟测速、策略分组状态概览
│   │   ├── composables/
│   │   │   └── useSettings.ts     # 多语言国际化 (中/英) 与主题管理
│   │   └── App.vue                # 布局骨架、全局样式与自定义滚动条定义
├── deploy/
│   ├── daex.service          # 生产 Systemd 单元模板 (自动探测真实配置)
│   ├── daex-dev-api.service  # 开发后端 Systemd 模板
│   └── daex-dev-web.service  # 开发前端 Systemd 模板
├── scripts/
│   ├── build-release.sh      # amd64 / arm64 交叉打包与 SHA-256 生成
│   └── doctor.sh             # 自动化环境与服务诊断脚本
├── install.sh                # 官方推荐一键安装脚本 (支持无缝迁移与静默安装)
├── uninstall.sh              # 干净卸载脚本
└── Makefile                  # 本地/远端部署快捷指令 (make deploy, make sync, make logs 等)
```

---

## 五、常用维护与操作指令

### 1. 本地代码同步与部署 (macOS 终端)
```bash
# 同步代码到远端测试机
make sync

# 编译并发布到生产环境 (自动打包前端并重启 daex.service)
make deploy

# 监控测试机运行日志
make logs

# 查看守护进程与网络状态
make status
```

### 2. 远端直接重新打包与发布 (在 192.168.3.10 上)
```bash
cd /opt/daex
. /etc/profile.d/daex-dev.sh
./scripts/build-release.sh v1.0.0
```

### 3. GitHub 远端代码与发布更新
```bash
# 提交代码并同步双分支
git push origin master && git push origin master:main

# 更新标签
git tag -fa v1.0.0 -m "daeX v1.0.0" && git push -f origin v1.0.0

# 覆盖上传 Release 资源
gh release upload v1.0.0 release/* --clobber
```

---

## 六、后续维护建议与注意事项

1. **真实配置修改安全性**：
   - 当前在 `ConfigView.vue` 修改并保存配置时会自动创建时间戳备份（位于 `/var/log/daex/backups`）。
   - 内核在热重载配置前会先经过 `dae validate` 验证，若语法有误将直接阻断并弹出错误行提示，避免导致透明代理网络中断。
2. **多节点订阅的更新机制**：
   - 如果未来新增了节点订阅定时自动更新功能，可借助后端定时器拉取，重新触发 `getNodesOverview` 刷新节点池与策略组匹配。
3. **保持双分支同步**：
   - GitHub 默认主分支为 `main`，部分开发者本地习惯推送到 `master`，因此在 `git push` 时应保持 `git push origin master && git push origin master:main` 同步。
