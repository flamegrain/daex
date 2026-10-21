#!/usr/bin/env bash
# ==============================================================================
# daeX 环境诊断与系统自检工具 (Doctor & Health Diagnostics)
# ==============================================================================

set -e

APP_NAME="daex"
DAE_BIN="/usr/local/bin/dae"
DEFAULT_PORT="9091"

# 输出着色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0

print_item() {
    local status="$1"
    local title="$2"
    local desc="$3"
    case "${status}" in
        PASS)
            echo -e "  [${GREEN}✓ PASS${NC}] ${BOLD}${title}${NC}: ${desc}"
            ((PASS_COUNT++)) || true
            ;;
        WARN)
            echo -e "  [${YELLOW}⚠ WARN${NC}] ${BOLD}${title}${NC}: ${YELLOW}${desc}${NC}"
            ((WARN_COUNT++)) || true
            ;;
        FAIL)
            echo -e "  [${RED}✗ FAIL${NC}] ${BOLD}${title}${NC}: ${RED}${desc}${NC}"
            ((FAIL_COUNT++)) || true
            ;;
        INFO)
            echo -e "  [${BLUE}ℹ INFO${NC}] ${BOLD}${title}${NC}: ${desc}"
            ;;
    esac
}

echo ""
echo -e "${CYAN}${BOLD}================================================================${NC}"
echo -e "${CYAN}${BOLD}       🔍 daeX 系统运行环境诊断与健康自检报告${NC}"
echo -e "${CYAN}${BOLD}================================================================${NC}"
echo ""

# 1. 检查 Linux 内核与 eBPF
echo -e "${BOLD}1. 操作系统与 eBPF 内核底层支持:${NC}"
OS_NAME=$(uname -s)
KERNEL_VER=$(uname -r)
ARCH=$(uname -m)

if [[ "${OS_NAME}" == "Linux" ]]; then
    print_item "PASS" "操作系统" "Linux ${KERNEL_VER} (${ARCH})"
else
    print_item "FAIL" "操作系统" "非 Linux 系统 (${OS_NAME})，dae 无法运行！"
fi

# 内核版本判断 (推荐 5.8+)
KERNEL_MAJOR=$(echo "${KERNEL_VER}" | cut -d. -f1)
KERNEL_MINOR=$(echo "${KERNEL_VER}" | cut -d. -f2)
if [[ "${KERNEL_MAJOR}" -gt 5 ]] || [[ "${KERNEL_MAJOR}" -eq 5 && "${KERNEL_MINOR}" -ge 8 ]]; then
    print_item "PASS" "内核版本要求" "当前内核 ${KERNEL_VER} 满足 eBPF 推荐要求 (>= 5.8)"
else
    print_item "WARN" "内核版本偏低" "当前内核 ${KERNEL_VER}，部分 eBPF 特性 (如 cgroup_skb / tc) 可能受限"
fi

# bpffs 挂载检查
if mount | grep -q "type bpf"; then
    print_item "PASS" "BPF 文件系统" "BPF fs 已正常挂载 (/sys/fs/bpf)"
elif [[ -d "/sys/fs/bpf" ]]; then
    print_item "WARN" "BPF 文件系统" "/sys/fs/bpf 存在但尚未以 bpf 类型挂载"
else
    print_item "FAIL" "BPF 文件系统" "未检测到 /sys/fs/bpf 挂载点"
fi

# 2. 检查 systemd 守护进程
echo ""
echo -e "${BOLD}2. systemd 服务管理器状态:${NC}"
if [[ -d /run/systemd/system ]] && command -v systemctl &>/dev/null; then
    SYS_STATE=$(systemctl is-system-running 2>/dev/null || echo "active")
    print_item "PASS" "systemd 状态" "systemd 正常运行 (${SYS_STATE})"
else
    print_item "FAIL" "systemd 状态" "未检测到活跃的 systemd 进程"
fi

# 3. 检查 dae 核心程序
echo ""
echo -e "${BOLD}3. dae 核心程序与底层服务:${NC}"
REAL_DAE_BIN=$(command -v dae 2>/dev/null || echo "/usr/local/bin/dae")
if [[ -x "${REAL_DAE_BIN}" ]]; then
    DAE_VER=$("${REAL_DAE_BIN}" --version 2>&1 | head -n 1 || echo "未知版本")
    print_item "PASS" "dae 核心程序" "已安装在 ${REAL_DAE_BIN} (${DAE_VER})"
else
    print_item "FAIL" "dae 核心程序" "未找到可执行的 dae 核心程序 (${REAL_DAE_BIN})"
fi

# dae.service 状态
if systemctl is-active --quiet dae 2>/dev/null; then
    DAE_PID=$(systemctl show dae --property=MainPID --value 2>/dev/null || echo "-")
    print_item "PASS" "dae.service 状态" "运行中 active (running), PID: ${DAE_PID}"
else
    DAE_STATE=$(systemctl is-active dae 2>/dev/null || echo "未安装/未运行")
    print_item "WARN" "dae.service 状态" "当前未运行 (${DAE_STATE})"
fi

# 4. 检查 dae 配置文件与语法
echo ""
echo -e "${BOLD}4. dae 配置文件与语法规则有效性:${NC}"
CONFIG_PATH=""
if systemctl list-unit-files dae.service &>/dev/null; then
    CONFIG_PATH=$(systemctl cat dae 2>/dev/null | grep -E '^ExecStart=' | tail -n 1 | grep -oE '(-c|--config)[ =]+[^ ]+' | awk '{print $2}' || true)
fi

if [[ -z "${CONFIG_PATH}" || ! -f "${CONFIG_PATH}" ]]; then
    for cand in "/usr/local/etc/dae/config.dae" "/etc/dae/config.dae" "/etc/daex/config.dae" "/opt/daex/config.mock.dae" "/etc/dae-webui/config.dae"; do
        if [[ -f "${cand}" ]]; then
            CONFIG_PATH="${cand}"
            break
        fi
    done
fi

if [[ -n "${CONFIG_PATH}" && -f "${CONFIG_PATH}" ]]; then
    # 检查权限
    PERMS=$(stat -c "%a" "${CONFIG_PATH}" 2>/dev/null || stat -f "%OLp" "${CONFIG_PATH}" 2>/dev/null || echo "000")
    if [[ "${PERMS}" == "600" || "${PERMS}" == "0600" ]]; then
        print_item "PASS" "配置文件权限" "${CONFIG_PATH} (安全权限 0600)"
    else
        print_item "WARN" "配置文件权限" "${CONFIG_PATH} 权限为 ${PERMS} (建议设为 0600 以保密代理凭据)"
    fi

    # 语法校验
    if [[ -x "${REAL_DAE_BIN}" ]]; then
        if "${REAL_DAE_BIN}" validate -c "${CONFIG_PATH}" &>/dev/null; then
            print_item "PASS" "配置语法校验" "dae validate 语法校验全部通过"
        else
            VAL_ERR=$("${REAL_DAE_BIN}" validate -c "${CONFIG_PATH}" 2>&1 | tail -n 3 | tr '\n' ' ' || echo "校验错误")
            print_item "FAIL" "配置语法校验" "语法存在错误: ${VAL_ERR}"
        fi
    fi
else
    print_item "FAIL" "配置文件检测" "未检测到可用的 dae 配置文件！"
fi

# 5. 检查路由规则资产 (GeoIP / GeoSite)
echo ""
echo -e "${BOLD}5. 分流规则资产数据库 (GeoIP / GeoSite):${NC}"
find_dat_file() {
    local name="$1"
    for dir in "/usr/local/share/dae" "/usr/share/dae" "/etc/dae" "/usr/local/etc/dae" "/var/lib/dae"; do
        if [[ -f "${dir}/${name}" ]]; then
            echo "${dir}/${name}"
            return 0
        fi
    done
    return 1
}

GEOSITE_FILE=$(find_dat_file "geosite.dat" || true)
GEOIP_FILE=$(find_dat_file "geoip.dat" || true)

if [[ -n "${GEOSITE_FILE}" ]]; then
    SIZE=$(ls -lh "${GEOSITE_FILE}" | awk '{print $5}')
    print_item "PASS" "geosite.dat" "已就绪 (${GEOSITE_FILE}, 大小: ${SIZE})"
else
    print_item "WARN" "geosite.dat" "未在标准目录检测到 geosite.dat，域名分流规则库可能缺失"
fi

if [[ -n "${GEOIP_FILE}" ]]; then
    SIZE=$(ls -lh "${GEOIP_FILE}" | awk '{print $5}')
    print_item "PASS" "geoip.dat" "已就绪 (${GEOIP_FILE}, 大小: ${SIZE})"
else
    print_item "WARN" "geoip.dat" "未在标准目录检测到 geoip.dat，IP 分流规则库可能缺失"
fi

# 6. 检查 daeX 安装与健康状态
echo ""
echo -e "${BOLD}6. daeX 控制台服务与端口状态:${NC}"
FOUND_BIN=""
if [[ -x "/usr/local/bin/daex" ]]; then
    FOUND_BIN="/usr/local/bin/daex"
elif [[ -x "/usr/local/bin/dae-webui" ]]; then
    FOUND_BIN="/usr/local/bin/dae-webui"
fi

if [[ -n "${FOUND_BIN}" ]]; then
    WEBUI_VER=$(${FOUND_BIN} -version 2>&1 || echo "installed")
    print_item "PASS" "daeX 主程序" "安装于 ${FOUND_BIN} (${WEBUI_VER})"
else
    print_item "WARN" "daeX 主程序" "/usr/local/bin/daex 未安装 (若使用 dev 服务可忽略)"
fi

# systemd 服务状态
if systemctl is-active --quiet daex 2>/dev/null; then
    WEBUI_PID=$(systemctl show daex --property=MainPID --value 2>/dev/null || echo "-")
    print_item "PASS" "daex.service" "运行中 active (running), PID: ${WEBUI_PID}"
elif systemctl is-active --quiet dae-webui 2>/dev/null; then
    WEBUI_PID=$(systemctl show dae-webui --property=MainPID --value 2>/dev/null || echo "-")
    print_item "PASS" "dae-webui.service (旧版兼容)" "运行中 active (running), PID: ${WEBUI_PID}"
elif systemctl is-active --quiet dae-webui-dev-api 2>/dev/null; then
    print_item "PASS" "daeX (DEV)" "开发模式服务运行中 (dae-webui-dev-api active)"
else
    print_item "WARN" "daex.service" "daex 系统服务未运行"
fi

# HTTP 健康检查探针
WEBUI_HEALTH=false
for p in 9091 8080 5173; do
    if curl -fsSL -m 2 "http://127.0.0.1:${p}/api/health" &>/dev/null; then
        print_item "PASS" "WebUI HTTP 探针" "HTTP 接口畅通 (http://127.0.0.1:${p}/api/health 响应 200 OK)"
        WEBUI_HEALTH=true
        break
    fi
done

if [[ "${WEBUI_HEALTH}" != "true" ]]; then
    print_item "WARN" "WebUI HTTP 探针" "未检测到本地 9091 / 8080 端口 HTTP 探针响应"
fi

# 7. 检查网络接口与默认路由
echo ""
echo -e "${BOLD}7. 网络接口与默认外网出口:${NC}"
DEF_ROUTE=$(ip route show default 2>/dev/null | head -n 1 || true)
if [[ -n "${DEF_ROUTE}" ]]; then
    DEF_DEV=$(echo "${DEF_ROUTE}" | grep -oE 'dev [^ ]+' | awk '{print $2}')
    DEF_GW=$(echo "${DEF_ROUTE}" | awk '{print $3}')
    print_item "PASS" "默认外网出口网卡" "${DEF_DEV} (网关: ${DEF_GW})"
else
    print_item "WARN" "默认外网路由" "未获取到默认路由网卡"
fi

# 8. 磁盘与数据目录可写性
echo ""
echo -e "${BOLD}8. 数据存储与备份目录权限:${NC}"
BACKUP_DIR="/var/log/daex/backups"
[[ ! -d "${BACKUP_DIR}" && -d "/var/log/dae-webui/backups" ]] && BACKUP_DIR="/var/log/dae-webui/backups"
mkdir -p "${BACKUP_DIR}" 2>/dev/null || true
if [[ -w "${BACKUP_DIR}" ]]; then
    AVAIL_DISK=$(df -h "${BACKUP_DIR}" 2>/dev/null | tail -n 1 | awk '{print $4}')
    print_item "PASS" "备份数据目录" "${BACKUP_DIR} (正常可写，可用磁盘空间: ${AVAIL_DISK})"
else
    print_item "WARN" "备份数据目录" "${BACKUP_DIR} 无写入权限"
fi

# 总结
echo ""
echo -e "${CYAN}${BOLD}================================================================${NC}"
TOTAL_CHECKS=$((PASS_COUNT + WARN_COUNT + FAIL_COUNT))
if [[ ${FAIL_COUNT} -eq 0 && ${WARN_COUNT} -eq 0 ]]; then
    echo -e "${GREEN}${BOLD}  🎉 诊断完成: 共检查 ${TOTAL_CHECKS} 项，全部通过 (PASS: ${PASS_COUNT})！系统环境非常健康！${NC}"
elif [[ ${FAIL_COUNT} -eq 0 ]]; then
    echo -e "${YELLOW}${BOLD}  ✨ 诊断完成: 共检查 ${TOTAL_CHECKS} 项 (通过: ${PASS_COUNT}, 警告: ${WARN_COUNT}, 失败: 0)。存在少许优化项。${NC}"
else
    echo -e "${RED}${BOLD}  ⚠️  诊断完成: 共检查 ${TOTAL_CHECKS} 项 (通过: ${PASS_COUNT}, 警告: ${WARN_COUNT}, 失败: ${FAIL_COUNT})。请优先修复上方标记 [FAIL] 的项目！${NC}"
fi
echo -e "${CYAN}${BOLD}================================================================${NC}"
echo ""

if [[ ${FAIL_COUNT} -gt 0 ]]; then
    exit 1
fi
exit 0
