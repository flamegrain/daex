#!/usr/bin/env bash
# ==============================================================================
# dae-webui 自动化卸载与清理脚本
# 项目主页: https://github.com/daeuniverse/dae-webui
# ==============================================================================

set -e

APP_NAME="dae-webui"
INSTALL_BIN="/usr/local/bin/${APP_NAME}"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
CONFIG_DIR="/etc/dae-webui"
LOG_BACKUP_DIR="/var/log/dae-webui"

# 输出着色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

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

check_root() {
    if [[ $EUID -ne 0 ]]; then
        fatal "此脚本必须以 root 权限运行，请尝试使用 'sudo $0 $*'"
    fi
}

show_help() {
    cat <<EOF
dae-webui 自动化卸载脚本

用法:
  $0 [选项]

选项:
  -y, --yes          非交互式运行，跳过所有确认提示
  --purge            彻底清理所有配置文件及历史备份 (${CONFIG_DIR}, ${LOG_BACKUP_DIR})
  --keep-data        保留配置与备份数据 (默认模式)
  --clean-dev        同时清理开发服务 (dae-webui-dev-api, dae-webui-dev-web)
  -h, --help         显示帮助信息

示例:
  sudo $0              # 交互式安全卸载 (保留配置)
  sudo $0 -y           # 一键静默卸载
  sudo $0 -y --purge   # 彻底卸载并清理所有相关文件
EOF
}

NON_INTERACTIVE="false"
PURGE_DATA="false"
CLEAN_DEV="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -y|--yes)
            NON_INTERACTIVE="true"
            shift
            ;;
        --purge)
            PURGE_DATA="true"
            shift
            ;;
        --keep-data)
            PURGE_DATA="false"
            shift
            ;;
        --clean-dev)
            CLEAN_DEV="true"
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            warn "未知参数: $1"
            shift
            ;;
    esac
done

check_root

echo ""
echo -e "${RED}${BOLD}================================================================${NC}"
echo -e "${RED}${BOLD}           ⚠️  dae-webui 卸载与清理向导${NC}"
echo -e "${RED}${BOLD}================================================================${NC}"
echo ""

if [[ "${NON_INTERACTIVE}" != "true" ]]; then
    read -r -p "确认停止并卸载 dae-webui 吗? [y/N]: " confirm_uninstall
    if [[ ! "${confirm_uninstall}" =~ ^[Yy]$ ]]; then
        info "用户取消卸载操作。"
        exit 0
    fi

    if [[ "${PURGE_DATA}" != "true" ]]; then
        read -r -p "是否同时彻底删除配置文件与日志备份数据 (${CONFIG_DIR}, ${LOG_BACKUP_DIR})? [y/N]: " confirm_purge
        if [[ "${confirm_purge}" =~ ^[Yy]$ ]]; then
            PURGE_DATA="true"
        fi
    fi
fi

# 1. 停止与注销生产 systemd 服务
info "[1/4] 停止并注销 dae-webui 系统服务..."
if systemctl is-active --quiet "${APP_NAME}" 2>/dev/null; then
    systemctl stop "${APP_NAME}" 2>/dev/null || true
    info "已停止正在运行的 ${APP_NAME} 服务"
fi

if systemctl is-enabled --quiet "${APP_NAME}" 2>/dev/null; then
    systemctl disable "${APP_NAME}" 2>/dev/null || true
    info "已关闭 ${APP_NAME} 开机自启"
fi

if [[ -f "${SYSTEMD_SERVICE_FILE}" ]]; then
    rm -f "${SYSTEMD_SERVICE_FILE}"
    info "已删除服务单元定义: ${SYSTEMD_SERVICE_FILE}"
fi

# 2. 清理开发服务 (如果存在或指定)
if [[ "${CLEAN_DEV}" == "true" ]] || [[ -f "/etc/systemd/system/dae-webui-dev-api.service" ]]; then
    info "[2/4] 清理关联开发服务 (dae-webui-dev-api, dae-webui-dev-web)..."
    systemctl stop dae-webui-dev-api dae-webui-dev-web 2>/dev/null || true
    systemctl disable dae-webui-dev-api dae-webui-dev-web 2>/dev/null || true
    rm -f /etc/systemd/system/dae-webui-dev-api.service /etc/systemd/system/dae-webui-dev-web.service
fi

systemctl daemon-reload
systemctl reset-failed 2>/dev/null || true

# 3. 移除可执行文件
info "[3/4] 移除主程序文件..."
if [[ -f "${INSTALL_BIN}" ]]; then
    rm -f "${INSTALL_BIN}"
    success "已移除主程序: ${INSTALL_BIN}"
else
    info "未检测到二进制文件: ${INSTALL_BIN}，跳过"
fi

# 4. 数据清理决策
info "[4/4] 配置文件与数据清理处理..."
if [[ "${PURGE_DATA}" == "true" ]]; then
    warn "执行彻底清理 (--purge)..."
    if [[ -d "${CONFIG_DIR}" ]]; then
        rm -rf "${CONFIG_DIR}"
        info "已清理配置目录: ${CONFIG_DIR}"
    fi
    if [[ -d "${LOG_BACKUP_DIR}" ]]; then
        rm -rf "${LOG_BACKUP_DIR}"
        info "已清理日志备份目录: ${LOG_BACKUP_DIR}"
    fi
    success "已彻底清除所有配置及日志备份数据！"
else
    info "数据已保留:"
    [[ -d "${CONFIG_DIR}" ]] && echo -e "  • 配置目录: ${CYAN}${CONFIG_DIR}${NC}"
    [[ -d "${LOG_BACKUP_DIR}" ]] && echo -e "  • 备份目录: ${CYAN}${LOG_BACKUP_DIR}${NC}"
    info "如需完全清理，可手动执行: rm -rf ${CONFIG_DIR} ${LOG_BACKUP_DIR}"
fi

echo ""
echo -e "${GREEN}================================================================${NC}"
echo -e "${GREEN}${BOLD}     🎉 dae-webui 卸载流程已成功完成！${NC}"
echo -e "${GREEN}================================================================${NC}"
echo -e "说明: 底层 dae 核心服务 (dae.service) 及其配置文件未受任何改动，仍保持正常运行。"
echo ""
