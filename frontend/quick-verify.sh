#!/bin/bash

# 快速验证脚本

echo "========================================="
echo "系统配置动态加载功能快速验证"
echo "========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 步骤 1: 检查数据库
echo -e "\n${YELLOW}步骤 1: 检查数据库${NC}"
if mysql -u root -p20041211wzwaicjW. -e "USE youdao" 2>/dev/null; then
    echo -e "${GREEN}✓ 数据库连接成功${NC}"
else
    echo -e "${RED}✗ 数据库连接失败${NC}"
    echo "请确保 MySQL 已启动且数据库 'youdao' 存在"
    exit 1
fi

# 步骤 2: 检查 sys_config 表
echo -e "\n${YELLOW}步骤 2: 检查 sys_config 表${NC}"
if mysql -u root -p20041211wzwaicjW. youdao -e "DESCRIBE sys_config" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ sys_config 表存在${NC}"
else
    echo -e "${RED}✗ sys_config 表不存在${NC}"
    echo "请先运行种子脚本: go run cmd/seed_sysconfig/main.go"
    exit 1
fi

# 步骤 3: 检查配置数据
echo -e "\n${YELLOW}步骤 3: 检查配置数据${NC}"
CONFIG_COUNT=$(mysql -u root -p20041211wzwaicjW. youdao -e "SELECT COUNT(*) FROM sys_config" 2>/dev/null | tail -1)
if [ "$CONFIG_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ 存在 $CONFIG_COUNT 条配置记录${NC}"
    echo "配置分组:"
    mysql -u root -p20041211wzwaicjW. youdao -e "SELECT DISTINCT config_group FROM sys_config" 2>/dev/null
else
    echo -e "${RED}✗ 没有配置记录${NC}"
    echo "请先运行种子脚本: go run cmd/seed_sysconfig/main.go"
    exit 1
fi

# 步骤 4: 检查后端服务
echo -e "\n${YELLOW}步骤 4: 检查后端服务${NC}"
if curl -s http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo -e "${GREEN}✓ 后端服务运行中${NC}"
else
    echo -e "${YELLOW}⚠ 后端服务未运行${NC}"
    echo "请在另一个终端运行: cd ../YoudaoNoteLM && go run cmd/server/main.go"
fi

# 步骤 5: 检查前端服务
echo -e "\n${YELLOW}步骤 5: 检查前端服务${NC}"
if curl -s http://localhost:5173 >/dev/null 2>&1; then
    echo -e "${GREEN}✓ 前端服务运行中${NC}"
else
    echo -e "${YELLOW}⚠ 前端服务未运行${NC}"
    echo "请在另一个终端运行: cd ../YouDaoNoteLM_Web && npm run dev"
fi

# 步骤 6: 测试 API
echo -e "\n${YELLOW}步骤 6: 测试 API${NC}"
if curl -s http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    # 尝试获取配置状态
    RESPONSE=$(curl -s http://localhost:8080/api/v1/admin/config/status 2>/dev/null)
    if echo "$RESPONSE" | grep -q '"code":0'; then
        echo -e "${GREEN}✓ API 接口正常${NC}"
    else
        echo -e "${YELLOW}⚠ API 接口需要认证${NC}"
        echo "请先登录获取 token"
    fi
else
    echo -e "${YELLOW}⚠ 跳过 API 测试（后端未运行）${NC}"
fi

echo -e "\n========================================="
echo "验证完成"
echo "========================================="
echo ""
echo "下一步操作:"
echo "1. 如果后端未运行，请运行: cd ../YoudaoNoteLM && go run cmd/server/main.go"
echo "2. 如果前端未运行，请运行: npm run dev"
echo "3. 访问 http://localhost:5173/admin 查看后台管理"
echo "4. 点击'系统配置'标签页查看配置"
echo ""
echo "详细验证步骤请参考: VALIDATION_GUIDE.md"