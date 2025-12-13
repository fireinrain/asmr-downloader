#!/bin/bash
# 构建脚本 - 支持主流三个平台（Linux、Windows、macOS）
# Usage: ./build.sh [version]

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目名称
PROJECT_NAME="asmroner"

# 默认版本号
VERSION=$(grep -oP 'version\s*=\s*"\K[^"]+' version.go || echo "v1.0.0")
BUILD_TIME=$(date +%Y-%m-%d_%H:%M:%S)
AUTHOR="fireinrain"

# 自定义版本号
if [ -n "$1" ]; then
  VERSION="$1"
fi

# 输出目录
OUTPUT_DIR="release"
mkdir -p "$OUTPUT_DIR"

echo -e "${GREEN}🚀 开始构建 $PROJECT_NAME${NC}"
echo -e "${YELLOW}📦 版本: $VERSION${NC}"
echo -e "${YELLOW}⏱️  构建时间: $BUILD_TIME${NC}"
echo -e "${YELLOW}👨‍💻 开发者: $AUTHOR${NC}"
echo "====================================="

# 清理旧的构建文件
echo -e "${YELLOW}🧹 清理旧的构建文件...${NC}"
rm -f "$PROJECT_NAME"-*-*

# 下载依赖
echo -e "${YELLOW}📥 下载依赖...${NC}"
go mod tidy

# 构建配置矩阵
BUILD_CONFIGS=(
  # OS    Arch    GOOS      GOARCH    Suffix
  "linux amd64   linux     amd64     "
  "windows amd64 windows  amd64     .exe"
  "macos amd64   darwin    amd64     "
  "macos arm64   darwin    arm64     "
)

# 执行构建
for config in "${BUILD_CONFIGS[@]}"; do
  read -r OS ARCH GOOS GOARCH SUFFIX <<< "$config"

  echo -e "\n${GREEN}🔨 构建 $OS/$ARCH 版本...${NC}"

  BINARY_NAME="${PROJECT_NAME}-${OS}-${ARCH}${SUFFIX}"

  # 构建命令
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.author=${AUTHOR}" \
    -o "$BINARY_NAME"

  if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 构建成功: $BINARY_NAME${NC}"

    # 创建发布包
    echo -e "${YELLOW}📦 创建发布包...${NC}"
    cp "$BINARY_NAME" "$OUTPUT_DIR/"

    # 根据平台创建压缩包
    if [[ "$OS" == "windows" ]]; then
      # Windows使用zip格式
      cd "$OUTPUT_DIR" || exit
      zip -r "../${PROJECT_NAME}-${OS}-${ARCH}-${VERSION}.zip" "$BINARY_NAME"
      cd ..
    else
      # Linux和macOS使用zip格式
      cd "$OUTPUT_DIR" || exit
      zip -r "../${PROJECT_NAME}-${OS}-${ARCH}-${VERSION}.zip" "$BINARY_NAME"
      cd ..
    fi

    echo -e "${GREEN}✅ 发布包创建成功: ${PROJECT_NAME}-${OS}-${ARCH}-${VERSION}.zip${NC}"
  else
    echo -e "${RED}❌ 构建失败: $BINARY_NAME${NC}"
  fi
done

# 清理临时文件
echo -e "\n${YELLOW}🧹 清理临时文件...${NC}"
rm -f "${PROJECT_NAME}"-*-*
rm -f "$OUTPUT_DIR/${PROJECT_NAME}"-*-*

echo -e "\n====================================="
echo -e "${GREEN}🎉 构建完成!${NC}"
echo -e "${YELLOW}📁 发布包位置:${NC}"
ls -la "${PROJECT_NAME}"-*-*.zip
echo -e "\n${YELLOW}📝 使用方法:${NC}"
echo -e "  ${PROJECT_NAME} --help"
echo -e "  ${PROJECT_NAME} config"
echo -e "  ${PROJECT_NAME} search RJ01037721"
echo -e "  ${PROJECT_NAME} download RJ01037721 -d ./downloads"