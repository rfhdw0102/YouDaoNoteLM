#!/bin/bash

# 系统配置动态加载功能测试脚本

echo "========================================="
echo "系统配置动态加载功能测试"
echo "========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 测试函数
run_test() {
    local test_name=$1
    local test_command=$2

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "\n${YELLOW}测试 $TOTAL_TESTS: $test_name${NC}"

    if eval "$test_command"; then
        echo -e "${GREEN}✓ 通过${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗ 失败${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# 检查后端是否运行
check_backend() {
    curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1
    return $?
}

# 检查前端是否运行
check_frontend() {
    curl -s http://localhost:5173 > /dev/null 2>&1
    return $?
}

# 检查数据库连接
check_database() {
    # 这里需要根据实际情况调整
    # 假设使用 MySQL 命令行工具
    mysql -u root -p20041211wzwaicjW. -e "SELECT 1" > /dev/null 2>&1
    return $?
}

# 获取管理员 token
get_admin_token() {
    # 这里需要根据实际情况调整
    # 假设使用 curl 调用登录接口
    local response=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@example.com","password":"password"}')

    echo "$response" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4
}

# 测试 1: 检查后端服务
run_test "检查后端服务是否运行" "check_backend"

# 测试 2: 检查前端服务
run_test "检查前端服务是否运行" "check_frontend"

# 测试 3: 检查数据库连接
run_test "检查数据库连接" "check_database"

# 测试 4: 检查系统配置表
run_test "检查系统配置表是否存在" "
    mysql -u root -p20041211wzwaicjW. youdao -e 'DESCRIBE sys_config' > /dev/null 2>&1
"

# 测试 5: 检查配置数据
run_test "检查配置数据是否存在" "
    mysql -u root -p20041211wzwaicjW. youdao -e 'SELECT COUNT(*) FROM sys_config' | grep -q '[1-9]'
"

# 测试 6: 获取管理员 token
echo -e "\n${YELLOW}测试 6: 获取管理员 token${NC}"
ADMIN_TOKEN=$(get_admin_token)
if [ -n "$ADMIN_TOKEN" ]; then
    echo -e "${GREEN}✓ 获取 token 成功${NC}"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo -e "${RED}✗ 获取 token 失败${NC}"
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi
TOTAL_TESTS=$((TOTAL_TESTS + 1))

# 测试 7: 测试 API 获取配置状态
run_test "测试 API 获取配置状态" "
    curl -s -H 'Authorization: Bearer $ADMIN_TOKEN' \
        http://localhost:8080/api/v1/admin/config/status | grep -q '\"code\":0'
"

# 测试 8: 测试 API 获取 ASR 配置
run_test "测试 API 获取 ASR 配置" "
    curl -s -H 'Authorization: Bearer $ADMIN_TOKEN' \
        http://localhost:8080/api/v1/admin/config/asr | grep -q '\"code\":0'
"

# 测试 9: 测试前端页面
run_test "测试前端后台管理页面" "
    curl -s http://localhost:5173/admin | grep -q 'admin'
"

# 输出测试结果
echo -e "\n========================================="
echo "测试结果统计"
echo "========================================="
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}所有测试通过！✓${NC}"
    exit 0
else
    echo -e "\n${RED}有测试失败！✗${NC}"
    exit 1
fi