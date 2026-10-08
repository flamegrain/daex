#!/usr/bin/env bash
# ==============================================================================
# dae-webui 自动化安装与运维部署脚本
# 项目主页: https://github.com/daeuniverse/dae
# 支持系统: Linux (systemd) | 架构: x86_64(amd64), aarch64(arm64)
# ==============================================================================

set -e

# 常量定义
APP_NAME="dae-webui"
APP_DESC="dae Transparent Proxy WebUI Gateway"
INSTALL_BIN_DIR="/usr/local/bin"
TARGET_BIN="${INSTALL_BIN_DIR}/${APP_NAME}"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
DEFAULT_PORT="9091"
DEFAULT_HOST="0.0.0.0"
DEFAULT_DAE_BIN="/usr/local/bin/dae"
DEFAULT_DAE_SERVICE="dae"
BACKUP_BASE_DIR="/var/log/dae-webui/backups"
GITHUB_REPO="daeuniverse/dae-webui"

# 输出着色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

fatal() {
    echo -e "${RED}[FATAL]${NC} $*" >&2
    exit 1
}

# 检查 root 权限
check_root() {
    if [[ $EUID -ne 0 ]]; then
        fatal "此脚本必须以 root 权限运行，请尝试使用 'sudo $0 $*'"
    fi
}

# 检查系统类型与 systemd
check_environment() {
    if [[ "$(uname -s)" != "Linux" ]]; then
        fatal "dae-webui 仅支持 Linux 操作系统！"
    fi

    if ! command -v systemctl &>/dev/null; then
        fatal "系统未检测到 systemd (systemctl)，dae-webui 依赖 systemd 进行服务托管与守护！"
    fi
}

# 架构检测
detect_arch() {
    local raw_arch
    raw_arch="$(uname -m)"
    case "${raw_arch}" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7*|armhf)
            ARCH="armv7"
            ;;
        *)
            fatal "暂不支持的 CPU 架构: ${raw_arch} (仅支持 amd64, arm64)"
            ;;
    esac
    info "检测到系统硬件架构: ${BOLD}${ARCH}${NC} (${raw_arch})"
}

# 检测 dae 真实服务与配置文件路径
detect_dae_config() {
    local detected_cfg=""

    # 1. 尝试从已有 dae.service 中提取配置路径
    if systemctl is-active --quiet dae 2>/dev/null || systemctl list-unit-files dae.service &>/dev/null; then
        local exec_line
        exec_line=$(systemctl cat dae 2>/dev/null | grep -E '^ExecStart=' | tail -n 1 || true)
        if [[ -n "${exec_line}" ]]; then
            # 匹配 -c <path> 或 --config <path>
            detected_cfg=$(echo "${exec_line}" | grep -oE '(-c|--config)[ =]+[^ ]+' | awk '{print $2}' || true)
        fi
    fi

    # 2. 如果未从 service 中提取到，尝试检测常见官方路径
    if [[ -z "${detected_cfg}" || ! -f "${detected_cfg}" ]]; then
        local candidates=(
            "/usr/local/etc/dae/config.dae"
            "/etc/dae/config.dae"
            "/opt/dae-webui/config.mock.dae"
            "/etc/dae-webui/config.dae"
        )
        for cand in "${candidates[@]}"; do
            if [[ -f "${cand}" ]]; then
                detected_cfg="${cand}"
                break
            fi
        done
    fi

    # 3. 如果还是没有，设置默认配置创建路径
    if [[ -z "${detected_cfg}" ]]; then
        detected_cfg="/etc/dae-webui/config.dae"
    fi

    DETECTED_CONFIG="${detected_cfg}"
}

# 自动生成初始空白 dae 配置文件
create_default_dae_config() {
    local target="$1"
    local dir
    dir="$(dirname "${target}")"
    mkdir -p "${dir}"

    if [[ ! -f "${target}" ]]; then
        info "正在初始化默认 dae 配置文件: ${target}"
        cat <<'EOF' > "${target}"
# dae 配置文件
global {
    lan_interface: eth0
    wan_interface: auto
    auto_config_kernel_parameter: true
    log_level: info
    tcp_check_url: 'http://cp.cloudflare.com'
}

node {
}

group {
    proxy {
        policy: min_moving_avg
    }
}

routing {
    pname(NetworkManager, systemd-resolved, dnsmasq) -> must_direct
    dip(224.0.0.0/3, 'ff00::/8') -> direct
    dip(geoip:private) -> direct
    fallback: proxy
}
EOF
        chmod 0600 "${target}"
        success "已生成初始配置文件: ${target}"
    fi
}

# 查找可用本机 IP
get_local_ip() {
    local ip
    ip=$(ip route get 1.1.1.1 2>/dev/null | grep -oE 'src [0-9.]+' | awk '{print $2}' || true)
    if [[ -z "${ip}" ]]; then
        ip=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
    fi
    if [[ -z "${ip}" ]]; then
        ip="127.0.0.1"
    fi
    echo "${ip}"
}

# 安装可执行文件
install_binary() {
    info "准备安装 dae-webui 主程序..."

    # 1. 优先检查当前运行目录是否已有编译好的 bin/dae-webui
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    if [[ -f "${script_dir}/bin/dae-webui" ]]; then
        info "使用当前源码目录已编译的二进制: ${script_dir}/bin/dae-webui"
        install -m 755 "${script_dir}/bin/dae-webui" "${TARGET_BIN}"
        return 0
    elif [[ -f "${script_dir}/dae-webui" ]]; then
        info "使用当前目录中的二进制: ${script_dir}/dae-webui"
        install -m 755 "${script_dir}/dae-webui" "${TARGET_BIN}"
        return 0
    fi

    # 2. 如果处于完整源码仓库且具备 Go 与 Node 环境，支持就地编译
    if [[ -f "${script_dir}/backend/main.go" && -f "${script_dir}/frontend/package.json" ]]; then
        if command -v go &>/dev/null && command -v pnpm &>/dev/null; then
            info "检测到源码目录及构建工具链，正在进行就地编译..."
            (
                cd "${script_dir}/frontend" && pnpm install --frozen-lockfile && pnpm build
                cd "${script_dir}/backend" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(date +%Y%m%d-%H%M)" -o "${script_dir}/bin/dae-webui" .
            )
            install -m 755 "${script_dir}/bin/dae-webui" "${TARGET_BIN}"
            success "编译并安装完成: ${TARGET_BIN}"
            return 0
        fi
    fi

    # 3. 若无本地二进制，则尝试从 GitHub Releases 下载预编译单文件包
    info "尝试从 GitHub 下载发布包 (${ARCH})..."
    local release_tag="${SPECIFIED_VERSION:-latest}"
    local download_url=""
    local tmp_dir
    tmp_dir="$(mktemp -d)"

    if [[ "${release_tag}" == "latest" ]]; then
        download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/dae-webui-linux-${ARCH}.tar.gz"
    else
        download_url="https://github.com/${GITHUB_REPO}/releases/download/${release_tag}/dae-webui-linux-${ARCH}.tar.gz"
    fi

    info "下载地址: ${download_url}"
    if curl -fL --progress-bar "${download_url}" -o "${tmp_dir}/dae-webui.tar.gz" 2>/dev/null || wget -q --show-progress "${download_url}" -O "${tmp_dir}/dae-webui.tar.gz" 2>/dev/null; then
        tar -zxf "${tmp_dir}/dae-webui.tar.gz" -C "${tmp_dir}"
        install -m 755 "${tmp_dir}/dae-webui" "${TARGET_BIN}"
        rm -rf "${tmp_dir}"
        success "预编译二进制已安装至: ${TARGET_BIN}"
        return 0
    fi

    rm -rf "${tmp_dir}"
    fatal "无法获取或编译 dae-webui 二进制，请检查网络或在源码目录中先运行 'make deploy'！"
}

# 配置 systemd 服务文件
configure_service() {
    local listen_addr="${CUSTOM_HOST:-$DEFAULT_HOST}:${CUSTOM_PORT:-$DEFAULT_PORT}"
    local cfg_path="${CUSTOM_CONFIG:-$DETECTED_CONFIG}"
    local dae_bin="${CUSTOM_DAE_BIN:-$DEFAULT_DAE_BIN}"
    local dae_service="${CUSTOM_DAE_SERVICE:-$DEFAULT_DAE_SERVICE}"

    # 检查若配置文件不存在则初始化
    if [[ ! -f "${cfg_path}" ]]; then
        create_default_dae_config "${cfg_path}"
    fi

    # 确保备份目录就绪
    local backup_dir="${CUSTOM_BACKUP_DIR:-$BACKUP_BASE_DIR}"
    mkdir -p "${backup_dir}"
    chmod 0700 "${backup_dir}"

    # 判断是否为系统目录 /usr/local/etc/dae
    local allow_system_arg=""
    if [[ "${cfg_path}" == /usr/local/etc/dae/* || "${cfg_path}" == "/usr/local/etc/dae" ]]; then
        allow_system_arg=" -allow-system-config"
    fi

    info "正在生成 systemd 服务文件: ${SYSTEMD_SERVICE_FILE}"

    cat <<EOF > "${SYSTEMD_SERVICE_FILE}"
[Unit]
Description=${APP_DESC}
After=network-online.target ${dae_service}.service
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=$(dirname "${cfg_path}")
ExecStart=${TARGET_BIN} -listen ${listen_addr} -dae-config ${cfg_path} -dae-bin ${dae_bin} -dae-service ${dae_service} -backup-dir ${backup_dir}${allow_system_arg}
Restart=on-failure
RestartSec=3s
LimitNOFILE=65535

# 安全与沙箱保护
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF

    chmod 0644 "${SYSTEMD_SERVICE_FILE}"
    systemctl daemon-reload
    success "systemd 服务文件已配置完成"
}

# 启动并检查服务
start_and_verify_service() {
    info "启动并启用 ${APP_NAME} 服务..."
    systemctl enable "${APP_NAME}"
    systemctl restart "${APP_NAME}"

    sleep 1.5

    if systemctl is-active --quiet "${APP_NAME}"; then
        success "${APP_NAME} 服务已成功运行！"
    else
        error "${APP_NAME} 启动失败，最近服务日志如下:"
        journalctl -u "${APP_NAME}" -n 20 --no-pager
        fatal "请检查上方日志排查启动错误！"
    fi
}

# 打印安装完成美观横幅
show_welcome_banner() {
    local host_ip
    host_ip="$(get_local_ip)"
    local port="${CUSTOM_PORT:-$DEFAULT_PORT}"
    local cfg_path="${CUSTOM_CONFIG:-$DETECTED_CONFIG}"

    echo ""
    echo -e "${GREEN}================================================================${NC}"
    echo -e "${GREEN}${BOLD}     🎉 恭喜！dae-webui 自动化安装与部署已圆满成功！${NC}"
    echo -e "${GREEN}================================================================${NC}"
    echo ""
    echo -e "  🌐 ${BOLD}控制台访问地址${NC} : ${CYAN}http://${host_ip}:${port}${NC}"
    echo -e "  🔑 ${BOLD}初始登录账户${NC}   : ${YELLOW}admin${NC}"
    echo -e "  🔒 ${BOLD}初始登录密码${NC}   : ${YELLOW}admin123${NC} (请进入系统设置及时修改)"
    echo -e "  📄 ${BOLD}关联配置文件${NC}   : ${BLUE}${cfg_path}${NC}"
    echo -e "  ⚙️  ${BOLD}核心底层服务${NC}   : ${BLUE}${CUSTOM_DAE_SERVICE:-$DEFAULT_DAE_SERVICE}.service${NC}"
    echo ""
    echo -e "${BOLD}常用管理命令:${NC}"
    echo -e "  • 查看服务状态: ${CYAN}systemctl status ${APP_NAME}${NC}"
    echo -e "  • 重启控制面板: ${CYAN}systemctl restart ${APP_NAME}${NC}"
    echo -e "  • 停止控制面板: ${CYAN}systemctl stop ${APP_NAME}${NC}"
    echo -e "  • 查看实时日志: ${CYAN}journalctl -u ${APP_NAME} -f${NC}"
    echo -e "  • 脚本快捷维护: ${CYAN}$0 [status|restart|logs|uninstall]${NC}"
    echo ""
    echo -e "${GREEN}================================================================${NC}"
}

# 卸载逻辑
do_uninstall() {
    check_root
    echo -e "${YELLOW}警告: 即将卸载 ${APP_NAME} 服务及相关二进制文件！${NC}"
    if [[ "${NON_INTERACTIVE}" != "true" ]]; then
        read -r -p "确认继续卸载吗? [y/N]: " confirm
        if [[ ! "${confirm}" =~ ^[Yy]$ ]]; then
            info "操作已取消"
            exit 0
        fi
    fi

    info "正在停止并清理 systemd 服务..."
    systemctl stop "${APP_NAME}" 2>/dev/null || true
    systemctl disable "${APP_NAME}" 2>/dev/null || true
    rm -f "${SYSTEMD_SERVICE_FILE}"
    systemctl daemon-reload

    info "正在移除二进制程序..."
    rm -f "${TARGET_BIN}"

    success "${APP_NAME} 服务与程序已彻底卸载！(配置文件与日志备份已保留)"
}

# 打印帮助信息
show_help() {
    cat <<EOF
dae-webui 自动化安装与运维部署脚本

用法:
  $0 [命令] [选项]

命令:
  install       安装并配置 dae-webui（默认命令）
  update        在线拉取或就地重新编译升级
  uninstall     卸载 dae-webui 服务及二进制
  start         启动服务
  stop          停止服务
  restart       重启服务
  status        查看服务运行状态
  logs          跟踪实时服务日志
  help, -h      显示此帮助信息

选项:
  --port <PORT>        自定义 WebUI 监听端口 (默认: 9091)
  --host <HOST>        自定义监听主机 IP (默认: 0.0.0.0)
  --config <PATH>      指定 dae 配置文件路径 (默认自动探测)
  --bin <PATH>         指定 dae 可执行文件路径 (默认: /usr/local/bin/dae)
  --service <NAME>     指定底层 dae systemd 服务名称 (默认: dae)
  --version <VERSION>  指定安装 Release 版本 (默认: latest)
  --no-start           安装后不立即启动
  -y, --yes            非交互式安装，无需手动确认

示例:
  $0 install --port 9091
  $0 install --config /usr/local/etc/dae/config.dae
  $0 restart
  $0 logs
EOF
}

# 参数解析
COMMAND="install"
NON_INTERACTIVE="false"
NO_START="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        install|update|uninstall|start|stop|restart|status|logs)
            COMMAND="$1"
            shift
            ;;
        --port)
            CUSTOM_PORT="$2"
            shift 2
            ;;
        --host)
            CUSTOM_HOST="$2"
            shift 2
            ;;
        --config)
            CUSTOM_CONFIG="$2"
            shift 2
            ;;
        --bin)
            CUSTOM_DAE_BIN="$2"
            shift 2
            ;;
        --service)
            CUSTOM_DAE_SERVICE="$2"
            shift 2
            ;;
        --version)
            SPECIFIED_VERSION="$2"
            shift 2
            ;;
        --no-start)
            NO_START="true"
            shift
            ;;
        -y|--yes)
            NON_INTERACTIVE="true"
            shift
            ;;
        -h|--help|help)
            show_help
            exit 0
            ;;
        *)
            warn "未知参数: $1"
            shift
            ;;
    esac
done

# 主流程路由
case "${COMMAND}" in
    install|update)
        check_root
        check_environment
        detect_arch
        detect_dae_config

        echo ""
        echo -e "${CYAN}${BOLD}>>> 开始执行 dae-webui 自动化安装与部署 <<<${NC}"
        info "目标可执行文件: ${TARGET_BIN}"
        info "自动匹配配置文件: ${CUSTOM_CONFIG:-$DETECTED_CONFIG}"
        info "监听端口: ${CUSTOM_PORT:-$DEFAULT_PORT}"
        echo ""

        install_binary
        configure_service

        if [[ "${NO_START}" != "true" ]]; then
            start_and_verify_service
            show_welcome_banner
        else
            success "安装完成 (未立即启动服务)"
        fi
        ;;

    uninstall)
        do_uninstall
        ;;

    start)
        check_root
        systemctl start "${APP_NAME}"
        success "${APP_NAME} 服务已启动"
        ;;

    stop)
        check_root
        systemctl stop "${APP_NAME}"
        success "${APP_NAME} 服务已停止"
        ;;

    restart)
        check_root
        systemctl restart "${APP_NAME}"
        success "${APP_NAME} 服务已重启"
        ;;

    status)
        systemctl --no-pager -l status "${APP_NAME}"
        ;;

    logs)
        journalctl -u "${APP_NAME}" -f -n 50
        ;;

    *)
        show_help
        ;;
esac
