#!/usr/bin/env bash
# ==============================================================================
# daeX 自动化打包发布脚本
# 构建跨平台发行包 (linux/amd64, linux/arm64)
# ==============================================================================

set -e

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/release"
VERSION="${1:-$(date +%Y%m%d-%H%M)}"

echo "===> 开始构建 daeX 发行版 [版本: ${VERSION}]"
mkdir -p "${DIST_DIR}"

# 1. 构建前端静态资源
echo "===> [1/4] 构建前端资源 (Vue 3 + Vite)..."
cd "${ROOT_DIR}/frontend"
pnpm install --frozen-lockfile
pnpm build

# 2. 编译 Linux amd64
echo "===> [2/4] 编译 Linux amd64 二进制..."
cd "${ROOT_DIR}/backend"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "${DIST_DIR}/daex-linux-amd64" .

# 3. 编译 Linux arm64
echo "===> [3/4] 编译 Linux arm64 二进制..."
cd "${ROOT_DIR}/backend"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "${DIST_DIR}/daex-linux-arm64" .

# 4. 打包压缩与校验
echo "===> [4/4] 生成 tar.gz 压缩包与 sha256 校验和..."
cd "${DIST_DIR}"
rm -f sha256sums.txt

for arch in amd64 arm64; do
    bin_name="daex-linux-${arch}"
    tar_name="daex-linux-${arch}.tar.gz"
    
    cp "${bin_name}" daex
    tar -czf "${tar_name}" daex
    rm -f daex
    sha256sum "${tar_name}" >> sha256sums.txt
    sha256sum "${bin_name}" >> sha256sums.txt
    echo "  - 已生成: ${tar_name}"

    # 同时保留兼容名称副本
    cp "${tar_name}" "dae-webui-linux-${arch}.tar.gz"
    cp "${bin_name}" "dae-webui-linux-${arch}"
    sha256sum "dae-webui-linux-${arch}.tar.gz" >> sha256sums.txt
done

echo ""
echo "=== 🎉 发行包构建完成！输出目录: ${DIST_DIR} ==="
ls -lh "${DIST_DIR}"
