#!/bin/bash

# 拓扑亲和性验证脚本
# 验证容器的 CPU 亲和性是否符合预期

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认参数
POD_NAME=""
EXPECTED_TOPOLOGY=""
TEST_LABEL=""
VERBOSE=false
DRY_RUN=false

# 显示帮助信息
show_help() {
    echo "拓扑亲和性验证脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -p, --pod POD_NAME         Pod 名称"
    echo "  -l, --label LABEL          测试标签 (验证所有相关 Pod)"
    echo "  -t, --topology TOPOLOGY    期望的拓扑级别 (NUMA|SOCKET|CLUSTER)"
    echo "  -v, --verbose              详细输出"
    echo "  -d, --dry-run             只显示检查结果，不返回错误码"
    echo "  -h, --help                显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 -p test-pod-123 -t NUMA"
    echo "  $0 -l test-2N-G-01 -t NUMA --verbose"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--pod)
            POD_NAME="$2"
            shift 2
            ;;
        -l|--label)
            TEST_LABEL="$2"
            shift 2
            ;;
        -t|--topology)
            EXPECTED_TOPOLOGY="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 未知选项: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# 检查参数
if [ -z "$POD_NAME" ] && [ -z "$TEST_LABEL" ]; then
    echo -e "${RED}❌ 必须指定 Pod 名称或测试标签${NC}"
    show_help
    exit 1
fi

if [ -z "$EXPECTED_TOPOLOGY" ]; then
    echo -e "${RED}❌ 必须指定期望的拓扑级别${NC}"
    show_help
    exit 1
fi

# 检查依赖
check_dependencies() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}❌ kubectl 未安装${NC}"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}❌ jq 未安装${NC}"
        exit 1
    fi
    
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}📦 依赖检查通过${NC}"
    fi
}

# 获取 Pod 信息
get_pod_info() {
    local pod_name="$1"
    kubectl get pod "$pod_name" -o json 2>/dev/null || echo "{}"
}

# 获取测试相关的 Pod
get_test_pods() {
    local label="$1"
    kubectl get pods -l "test-label=${label}" -o json 2>/dev/null || echo "{\"items\":[]}"
}

# 获取 Pod 的 CPU 亲和性信息
get_pod_cpu_affinity() {
    local pod_name="$1"
    
    # 方法1: 检查 Pod 的 cpuset 配置
    local cpuset_info=""
    if kubectl exec "$pod_name" -- test -f /proc/self/status 2>/dev/null; then
        cpuset_info=$(kubectl exec "$pod_name" -- cat /proc/self/status 2>/dev/null | grep "Cpus_allowed_list" | awk '{print $2}' || echo "")
    fi
    
    # 方法2: 检查 Pod 的调度注解
    local scheduling_info=$(kubectl get pod "$pod_name" -o jsonpath='{.metadata.annotations}' 2>/dev/null || echo "{}")
    
    # 方法3: 检查 kunpeng-tap 相关的注解或标签
    local tap_annotations=$(echo "$scheduling_info" | jq -r 'to_entries[] | select(.key | contains("kunpeng")) | "\(.key)=\(.value)"' 2>/dev/null || echo "")
    
    echo "cpuset:$cpuset_info|annotations:$tap_annotations"
}

# 分析 CPU 亲和性
analyze_cpu_affinity() {
    local affinity_info="$1"
    local expected_topology="$2"
    
    # 提取 cpuset 信息
    local cpuset=$(echo "$affinity_info" | cut -d'|' -f1 | cut -d':' -f2)
    local annotations=$(echo "$affinity_info" | cut -d'|' -f2 | cut -d':' -f2)
    
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}🔍 CPU 亲和性信息:${NC}"
        echo -e "  cpuset: $cpuset"
        echo -e "  annotations: $annotations"
    fi
    
    # 分析 cpuset 范围
    local numa_analysis=$(analyze_cpuset_numa "$cpuset")
    local socket_analysis=$(analyze_cpuset_socket "$cpuset")
    
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}📊 拓扑分析:${NC}"
        echo -e "  NUMA 分析: $numa_analysis"
        echo -e "  SOCKET 分析: $socket_analysis"
    fi
    
    # 验证是否符合期望的拓扑级别
    case "$expected_topology" in
        "NUMA")
            if [[ "$numa_analysis" == "single_numa" ]]; then
                echo -e "${GREEN}✅ CPU 亲和性符合 NUMA 级别要求${NC}"
                return 0
            else
                echo -e "${RED}❌ CPU 亲和性不符合 NUMA 级别要求: $numa_analysis${NC}"
                return 1
            fi
            ;;
        "SOCKET")
            if [[ "$socket_analysis" == "single_socket" ]]; then
                echo -e "${GREEN}✅ CPU 亲和性符合 SOCKET 级别要求${NC}"
                return 0
            else
                echo -e "${RED}❌ CPU 亲和性不符合 SOCKET 级别要求: $socket_analysis${NC}"
                return 1
            fi
            ;;
        "CROSS_NUMA")
            if [[ "$numa_analysis" == "cross_numa" ]]; then
                echo -e "${GREEN}✅ CPU 亲和性符合跨 NUMA 要求${NC}"
                return 0
            else
                echo -e "${RED}❌ CPU 亲和性不符合跨 NUMA 要求: $numa_analysis${NC}"
                return 1
            fi
            ;;
        "CROSS_SOCKET")
            if [[ "$socket_analysis" == "cross_socket" ]]; then
                echo -e "${GREEN}✅ CPU 亲和性符合跨 SOCKET 要求${NC}"
                return 0
            else
                echo -e "${RED}❌ CPU 亲和性不符合跨 SOCKET 要求: $socket_analysis${NC}"
                return 1
            fi
            ;;
        "CLUSTER")
            echo -e "${GREEN}✅ 集群级别调度，无特定亲和性要求${NC}"
            return 0
            ;;
        *)
            echo -e "${RED}❌ 未知的拓扑级别: $expected_topology${NC}"
            return 1
            ;;
    esac
}

# 分析 cpuset 的 NUMA 分布
analyze_cpuset_numa() {
    local cpuset="$1"
    
    if [ -z "$cpuset" ] || [ "$cpuset" = "unknown" ]; then
        echo "unknown"
        return
    fi
    
    # 解析 CPU 范围 (例如: 0-23,48-71 或 0,1,2,3)
    local numa_nodes=()
    
    # 简化的 NUMA 分析 (假设 2NUMA-48核配置)
    # NUMA#0: 0-47, NUMA#1: 48-95
    local numa0_found=false
    local numa1_found=false
    
    # 检查是否包含 NUMA0 的 CPU
    if [[ "$cpuset" =~ [0-4][0-7]? ]] || [[ "$cpuset" =~ [0-3][0-9] ]]; then
        numa0_found=true
    fi
    
    # 检查是否包含 NUMA1 的 CPU
    if [[ "$cpuset" =~ [4-9][0-9] ]]; then
        numa1_found=true
    fi
    
    if [ "$numa0_found" = true ] && [ "$numa1_found" = true ]; then
        echo "cross_numa"
    elif [ "$numa0_found" = true ] || [ "$numa1_found" = true ]; then
        echo "single_numa"
    else
        echo "unknown"
    fi
}

# 分析 cpuset 的 SOCKET 分布
analyze_cpuset_socket() {
    local cpuset="$1"
    
    if [ -z "$cpuset" ] || [ "$cpuset" = "unknown" ]; then
        echo "unknown"
        return
    fi
    
    # 简化的 SOCKET 分析 (假设 4NUMA-24核配置)
    # SOCKET#0: 0-47, SOCKET#1: 48-95
    local socket0_found=false
    local socket1_found=false
    
    # 检查是否包含 SOCKET0 的 CPU
    if [[ "$cpuset" =~ [0-4][0-7]? ]]; then
        socket0_found=true
    fi
    
    # 检查是否包含 SOCKET1 的 CPU
    if [[ "$cpuset" =~ [4-9][0-9] ]]; then
        socket1_found=true
    fi
    
    if [ "$socket0_found" = true ] && [ "$socket1_found" = true ]; then
        echo "cross_socket"
    elif [ "$socket0_found" = true ] || [ "$socket1_found" = true ]; then
        echo "single_socket"
    else
        echo "unknown"
    fi
}

# 验证单个 Pod
verify_single_pod() {
    local pod_name="$1"
    local expected_topology="$2"
    
    echo -e "${BLUE}🔍 验证 Pod: $pod_name${NC}"
    
    # 检查 Pod 是否存在
    local pod_info=$(get_pod_info "$pod_name")
    if [ "$(echo "$pod_info" | jq -r '.metadata.name // empty')" != "$pod_name" ]; then
        echo -e "${RED}❌ Pod $pod_name 不存在${NC}"
        return 1
    fi
    
    # 检查 Pod 状态
    local pod_phase=$(echo "$pod_info" | jq -r '.status.phase')
    if [ "$pod_phase" != "Running" ]; then
        echo -e "${YELLOW}⚠️ Pod $pod_name 状态不是 Running: $pod_phase${NC}"
        if [ "$DRY_RUN" = false ]; then
            return 1
        fi
    fi
    
    # 获取 CPU 亲和性信息
    local affinity_info=$(get_pod_cpu_affinity "$pod_name")
    
    # 分析亲和性
    analyze_cpu_affinity "$affinity_info" "$expected_topology"
}

# 验证多个 Pod
verify_multiple_pods() {
    local test_label="$1"
    local expected_topology="$2"
    
    echo -e "${BLUE}🔍 验证测试标签: $test_label${NC}"
    
    # 获取测试 Pod
    local pods_json=$(get_test_pods "$test_label")
    local pod_count=$(echo "$pods_json" | jq -r '.items | length')
    
    if [ "$pod_count" -eq 0 ]; then
        echo -e "${YELLOW}⚠️ 没有找到标签为 ${test_label} 的 Pod${NC}"
        return 1
    fi
    
    echo -e "${BLUE}📊 找到 ${pod_count} 个测试 Pod${NC}"
    
    local success_count=0
    local total_count=0
    
    # 验证每个 Pod
    for i in $(seq 0 $((pod_count - 1))); do
        local pod_name=$(echo "$pods_json" | jq -r ".items[$i].metadata.name")
        total_count=$((total_count + 1))
        
        echo ""
        if verify_single_pod "$pod_name" "$expected_topology"; then
            success_count=$((success_count + 1))
        fi
    done
    
    echo ""
    echo -e "${BLUE}📈 验证结果统计:${NC}"
    echo -e "  成功: ${success_count}/${total_count} 个 Pod"
    echo -e "  成功率: $(echo "scale=1; $success_count * 100 / $total_count" | bc)%"
    
    if [ "$success_count" -eq "$total_count" ]; then
        echo -e "${GREEN}✅ 所有 Pod 的拓扑亲和性验证通过${NC}"
        return 0
    else
        echo -e "${RED}❌ 部分 Pod 的拓扑亲和性验证失败${NC}"
        if [ "$DRY_RUN" = false ]; then
            return 1
        else
            return 0
        fi
    fi
}

# 主函数
main() {
    echo -e "${BLUE}🔍 拓扑亲和性验证${NC}"
    echo -e "期望拓扑级别: ${EXPECTED_TOPOLOGY}"
    echo ""
    
    check_dependencies
    
    if [ -n "$POD_NAME" ]; then
        verify_single_pod "$POD_NAME" "$EXPECTED_TOPOLOGY"
    elif [ -n "$TEST_LABEL" ]; then
        verify_multiple_pods "$TEST_LABEL" "$EXPECTED_TOPOLOGY"
    fi
}

# 如果直接执行脚本
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
