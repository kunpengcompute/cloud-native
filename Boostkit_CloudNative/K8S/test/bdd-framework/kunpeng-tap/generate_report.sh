#!/bin/bash

# kunpeng-tap BDD 测试报告生成脚本
# 用法: ./generate_report.sh [test_type]
# test_type: all, smoke, topology, numa, bulk, mixed, boundary, failure

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 打印标题
print_header() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                      ║"
    echo "║           kunpeng-tap BDD 测试报告生成器                            ║"
    echo "║                                                                      ║"
    echo "╚══════════════════════════════════════════════════════════════════════╝"
    echo ""
}

# 检查依赖
check_dependencies() {
    print_info "检查依赖..."
    
    # 检查 Python
    if ! command -v python3 &> /dev/null; then
        print_error "Python 3 未安装"
        exit 1
    fi
    
    # 检查 pytest
    if ! python3 -c "import pytest" &> /dev/null; then
        print_error "pytest 未安装，请运行: pip install pytest pytest-bdd pytest-html"
        exit 1
    fi
    
    # 检查 pytest-html
    if ! python3 -c "import pytest_html" &> /dev/null; then
        print_warning "pytest-html 未安装，将安装..."
        pip install pytest-html
    fi
    
    print_success "依赖检查完成"
}

# 获取测试类型
TEST_TYPE=${1:-all}

# 定义测试命令
case $TEST_TYPE in
    all)
        TEST_CMD="pytest -v"
        REPORT_NAME="kunpeng-tap-full-report"
        TEST_DESC="完整测试套件"
        ;;
    smoke)
        TEST_CMD="pytest -m smoke -v"
        REPORT_NAME="kunpeng-tap-smoke-report"
        TEST_DESC="冒烟测试"
        ;;
    topology)
        TEST_CMD="pytest test_topology_aware.py -v"
        REPORT_NAME="kunpeng-tap-topology-report"
        TEST_DESC="拓扑感知测试 (topology-aware)"
        ;;
    numa)
        TEST_CMD="pytest test_numa_aware.py -v"
        REPORT_NAME="kunpeng-tap-numa-report"
        TEST_DESC="NUMA 感知测试 (numa-aware)"
        ;;
    bulk)
        TEST_CMD="pytest test_bulk_deployment.py -v"
        REPORT_NAME="kunpeng-tap-bulk-report"
        TEST_DESC="批量部署测试"
        ;;
    mixed)
        TEST_CMD="pytest test_mixed_deployment.py -v"
        REPORT_NAME="kunpeng-tap-mixed-report"
        TEST_DESC="混合部署测试"
        ;;
    boundary)
        TEST_CMD="pytest test_boundary_condition.py -v"
        REPORT_NAME="kunpeng-tap-boundary-report"
        TEST_DESC="边界条件测试"
        ;;
    failure)
        TEST_CMD="pytest test_failure_recovery.py -v"
        REPORT_NAME="kunpeng-tap-failure-report"
        TEST_DESC="故障恢复测试"
        ;;
    *)
        print_error "未知的测试类型: $TEST_TYPE"
        echo "用法: $0 [all|smoke|topology|numa|bulk|mixed|boundary|failure]"
        exit 1
        ;;
esac

# 创建报告目录
REPORT_DIR="test-reports"
mkdir -p $REPORT_DIR

# 生成时间戳
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
HTML_REPORT="${REPORT_DIR}/${REPORT_NAME}_${TIMESTAMP}.html"
JUNIT_REPORT="${REPORT_DIR}/${REPORT_NAME}_${TIMESTAMP}.xml"

# 打印测试信息
print_header
print_info "测试类型: ${TEST_DESC}"
print_info "测试命令: ${TEST_CMD}"
print_info "HTML 报告: ${HTML_REPORT}"
print_info "JUnit 报告: ${JUNIT_REPORT}"
echo ""

# 检查依赖
check_dependencies

# 运行测试
print_info "开始运行测试..."
echo ""

# 获取系统信息
HOSTNAME=$(hostname)
KUBE_VERSION=$(kubectl version --short 2>/dev/null | grep Server || echo "Unknown")
PYTHON_VERSION=$(python3 --version)

# 运行测试并生成报告
$TEST_CMD \
    --html=$HTML_REPORT \
    --self-contained-html \
    --junitxml=$JUNIT_REPORT \
    --metadata "测试类型" "${TEST_DESC}" \
    --metadata "主机名" "${HOSTNAME}" \
    --metadata "Kubernetes" "${KUBE_VERSION}" \
    --metadata "Python" "${PYTHON_VERSION}" \
    --metadata "测试时间" "$(date '+%Y-%m-%d %H:%M:%S')" \
    --metadata "测试者" "${USER}" \
    || TEST_RESULT=$?

echo ""

# 检查测试结果
if [ -z "$TEST_RESULT" ] || [ $TEST_RESULT -eq 0 ]; then
    print_success "测试全部通过！"
    EXIT_CODE=0
else
    print_warning "部分测试失败，请查看报告"
    EXIT_CODE=$TEST_RESULT
fi

# 生成简要报告
print_info "生成简要报告..."

# 提取测试统计信息
if [ -f "$JUNIT_REPORT" ]; then
    TOTAL_TESTS=$(grep -oP 'tests="\K[0-9]+' $JUNIT_REPORT | head -1)
    FAILURES=$(grep -oP 'failures="\K[0-9]+' $JUNIT_REPORT | head -1)
    ERRORS=$(grep -oP 'errors="\K[0-9]+' $JUNIT_REPORT | head -1)
    SKIPPED=$(grep -oP 'skipped="\K[0-9]+' $JUNIT_REPORT | head -1)
    TIME=$(grep -oP 'time="\K[0-9.]+' $JUNIT_REPORT | head -1)
    
    PASSED=$((TOTAL_TESTS - FAILURES - ERRORS - SKIPPED))
    
    # 打印统计信息
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════╗"
    echo "║                         测试结果统计                                 ║"
    echo "╠══════════════════════════════════════════════════════════════════════╣"
    printf "║  总测试数: %-56s ║\n" "$TOTAL_TESTS"
    printf "║  通过: %-60s ║\n" "$PASSED"
    printf "║  失败: %-60s ║\n" "$FAILURES"
    printf "║  错误: %-60s ║\n" "$ERRORS"
    printf "║  跳过: %-60s ║\n" "$SKIPPED"
    printf "║  耗时: %-56s 秒 ║\n" "$TIME"
    echo "╠══════════════════════════════════════════════════════════════════════╣"
    printf "║  HTML 报告: %-54s ║\n" "$(basename $HTML_REPORT)"
    printf "║  JUnit 报告: %-53s ║\n" "$(basename $JUNIT_REPORT)"
    echo "╚══════════════════════════════════════════════════════════════════════╝"
    echo ""
fi

# 创建最新报告的符号链接
ln -sf $(basename $HTML_REPORT) ${REPORT_DIR}/${REPORT_NAME}-latest.html
ln -sf $(basename $JUNIT_REPORT) ${REPORT_DIR}/${REPORT_NAME}-latest.xml

print_success "报告生成完成！"
echo ""
print_info "查看 HTML 报告:"
echo "  firefox $HTML_REPORT"
echo "  或"
echo "  chrome $HTML_REPORT"
echo ""
print_info "查看最新报告:"
echo "  firefox ${REPORT_DIR}/${REPORT_NAME}-latest.html"
echo ""

# 如果在 CI 环境中，输出报告路径
if [ -n "$CI" ]; then
    echo "::set-output name=html_report::$HTML_REPORT"
    echo "::set-output name=junit_report::$JUNIT_REPORT"
fi

exit $EXIT_CODE

