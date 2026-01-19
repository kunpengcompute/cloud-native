#!/bin/bash

# NUMA 均衡度检查脚本
# 检查 /sys/fs/cgroup/cpuset 下各 NUMA 的容器分布是否均衡

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认参数
TEST_LABEL=""
TOLERANCE=0.2
VERBOSE=false
DRY_RUN=false

# 显示帮助信息
show_help() {
    echo "NUMA 均衡度检查脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -l, --label LABEL       测试标签 (必需)"
    echo "  -t, --tolerance FLOAT   允许的不均衡度 (默认: 0.2)"
    echo "  -v, --verbose           详细输出"
    echo "  -d, --dry-run          只显示检查结果，不返回错误码"
    echo "  -h, --help             显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 -l test-2N-G-01 -t 0.3"
    echo "  $0 --label bulk-test-BULK-2N-01 --verbose"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -l|--label)
            TEST_LABEL="$2"
            shift 2
            ;;
        -t|--tolerance)
            TOLERANCE="$2"
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

# 检查必需参数
if [ -z "$TEST_LABEL" ]; then
    echo -e "${RED}❌ 缺少测试标签参数${NC}"
    show_help
    exit 1
fi

# 检查依赖
check_dependencies() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}❌ kubectl 未安装${NC}"
        exit 1
    fi
    
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}📦 依赖检查通过${NC}"
    fi
}

# 获取测试相关的 Pod
get_test_pods() {
    local label="$1"
    kubectl get pods -l "test-label=${label}" -o json 2>/dev/null || echo "{\"items\":[]}"
}

# 获取 Pod 的 NUMA 分配信息
get_pod_numa_info() {
    local pod_name="$1"
    local node_name="$2"
    
    # 获取 Pod 的 cpuset 信息
    # 这里需要根据实际的 kunpeng-tap 实现来获取 NUMA 分配信息
    # 简化实现：通过 Pod 的调度节点和资源配置推断 NUMA 分配
    
    # 方法1: 通过 kubectl exec 检查容器内的 cpuset
    local cpuset_info=""
    if kubectl exec "$pod_name" -- cat /proc/self/status 2>/dev/null | grep -q "Cpus_allowed_list"; then
        cpuset_info=$(kubectl exec "$pod_name" -- cat /proc/self/status 2>/dev/null | grep "Cpus_allowed_list" | awk '{print $2}')
    fi
    
    # 方法2: 通过节点上的 cgroup 信息检查
    # 这需要在节点上执行，这里简化处理
    
    # 方法3: 通过 kunpeng-tap 的日志或状态检查
    # 这需要根据 kunpeng-tap 的具体实现
    
    # 简化实现：根据 CPU 范围推断 NUMA
    if [ -n "$cpuset_info" ]; then
        echo "$cpuset_info"
    else
        echo "unknown"
    fi
}

# 将 CPU 列表转换为 NUMA 节点
cpu_list_to_numa() {
    local cpu_list="$1"
    local machine_config="$2"
    
    # 根据机器配置将 CPU 列表映射到 NUMA 节点
    # 这里需要根据实际的机器配置来实现
    
    case "$machine_config" in
        "2numa_48")
            # NUMA#0: 0-47, NUMA#1: 48-95
            if [[ "$cpu_list" =~ ^[0-4][0-7]?$ ]] || [[ "$cpu_list" =~ ^[0-3][0-9]$ ]]; then
                echo "numa0"
            elif [[ "$cpu_list" =~ ^[4-9][0-9]$ ]]; then
                echo "numa1"
            else
                echo "mixed"
            fi
            ;;
        "4numa_24")
            # NUMA#0: 0-23, NUMA#1: 24-47, NUMA#2: 48-71, NUMA#3: 72-95
            if [[ "$cpu_list" =~ ^[0-1][0-9]$ ]] || [[ "$cpu_list" =~ ^2[0-3]$ ]]; then
                echo "numa0"
            elif [[ "$cpu_list" =~ ^[2-3][0-9]$ ]] || [[ "$cpu_list" =~ ^4[0-7]$ ]]; then
                echo "numa1"
            elif [[ "$cpu_list" =~ ^[4-6][0-9]$ ]] || [[ "$cpu_list" =~ ^7[0-1]$ ]]; then
                echo "numa2"
            elif [[ "$cpu_list" =~ ^[7-9][0-9]$ ]]; then
                echo "numa3"
            else
                echo "mixed"
            fi
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# 统计 NUMA 分布
count_numa_distribution() {
    local test_label="$1"
    
    # 获取测试 Pod
    local pods_json=$(get_test_pods "$test_label")
    local pod_count=$(echo "$pods_json" | jq -r '.items | length')
    
    if [ "$pod_count" -eq 0 ]; then
        echo -e "${YELLOW}⚠️ 没有找到标签为 ${test_label} 的 Pod${NC}"
        return 1
    fi
    
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}📊 找到 ${pod_count} 个测试 Pod${NC}"
    fi
    
    # 初始化 NUMA 计数器
    declare -A numa_count
    numa_count["numa0"]=0
    numa_count["numa1"]=0
    numa_count["numa2"]=0
    numa_count["numa3"]=0
    numa_count["mixed"]=0
    numa_count["unknown"]=0
    
    # 遍历每个 Pod
    for i in $(seq 0 $((pod_count - 1))); do
        local pod_name=$(echo "$pods_json" | jq -r ".items[$i].metadata.name")
        local node_name=$(echo "$pods_json" | jq -r ".items[$i].spec.nodeName")
        local pod_phase=$(echo "$pods_json" | jq -r ".items[$i].status.phase")
        
        if [ "$pod_phase" != "Running" ]; then
            if [ "$VERBOSE" = true ]; then
                echo -e "${YELLOW}⚠️ Pod ${pod_name} 状态不是 Running: ${pod_phase}${NC}"
            fi
            continue
        fi
        
        # 获取 Pod 的 NUMA 信息
        local cpu_info=$(get_pod_numa_info "$pod_name" "$node_name")
        local numa_node=$(cpu_list_to_numa "$cpu_info" "2numa_48")  # 这里需要动态获取机器配置
        
        # 增加对应 NUMA 的计数
        numa_count["$numa_node"]=$((numa_count["$numa_node"] + 1))
        
        if [ "$VERBOSE" = true ]; then
            echo -e "${BLUE}📍 Pod ${pod_name}: CPU=${cpu_info}, NUMA=${numa_node}${NC}"
        fi
    done
    
    # 输出分布统计
    echo -e "${BLUE}📊 NUMA 分布统计:${NC}"
    local total_pods=0
    for numa in numa0 numa1 numa2 numa3; do
        local count=${numa_count["$numa"]}
        if [ "$count" -gt 0 ]; then
            echo -e "  ${numa}: ${count} 个 Pod"
            total_pods=$((total_pods + count))
        fi
    done
    
    if [ ${numa_count["mixed"]} -gt 0 ]; then
        echo -e "  mixed: ${numa_count["mixed"]} 个 Pod (跨 NUMA)"
        total_pods=$((total_pods + numa_count["mixed"]))
    fi
    
    if [ ${numa_count["unknown"]} -gt 0 ]; then
        echo -e "  unknown: ${numa_count["unknown"]} 个 Pod (未知)"
        total_pods=$((total_pods + numa_count["unknown"]))
    fi
    
    echo -e "  总计: ${total_pods} 个 Pod"
    
    # 计算均衡度
    calculate_balance_score "${numa_count[@]}"
}

# 计算均衡度分数
calculate_balance_score() {
    local -A numa_count
    numa_count["numa0"]=$1
    numa_count["numa1"]=$2
    numa_count["numa2"]=$3
    numa_count["numa3"]=$4
    
    # 只考虑有 Pod 的 NUMA 节点
    local active_numas=()
    local total_pods=0
    
    for numa in numa0 numa1 numa2 numa3; do
        local count=${numa_count["$numa"]}
        if [ "$count" -gt 0 ]; then
            active_numas+=("$count")
            total_pods=$((total_pods + count))
        fi
    done
    
    if [ ${#active_numas[@]} -eq 0 ] || [ "$total_pods" -eq 0 ]; then
        echo -e "${YELLOW}⚠️ 没有有效的 NUMA 分布数据${NC}"
        return 1
    fi
    
    # 计算平均值
    local avg=$(echo "scale=2; $total_pods / ${#active_numas[@]}" | bc)
    
    # 计算最大偏差
    local max_deviation=0
    for count in "${active_numas[@]}"; do
        local deviation=$(echo "scale=2; if ($count - $avg < 0) $avg - $count else $count - $avg" | bc)
        if (( $(echo "$deviation > $max_deviation" | bc -l) )); then
            max_deviation=$deviation
        fi
    done
    
    # 计算不均衡度
    local imbalance_ratio=0
    if (( $(echo "$avg > 0" | bc -l) )); then
        imbalance_ratio=$(echo "scale=4; $max_deviation / $avg" | bc)
    fi
    
    echo -e "${BLUE}📈 均衡度分析:${NC}"
    echo -e "  平均每个 NUMA: ${avg} 个 Pod"
    echo -e "  最大偏差: ${max_deviation}"
    echo -e "  不均衡度: ${imbalance_ratio} (阈值: ${TOLERANCE})"
    
    # 判断是否均衡
    if (( $(echo "$imbalance_ratio <= $TOLERANCE" | bc -l) )); then
        echo -e "${GREEN}✅ NUMA 分布均衡${NC}"
        return 0
    else
        echo -e "${RED}❌ NUMA 分布不均衡${NC}"
        if [ "$DRY_RUN" = false ]; then
            return 1
        else
            return 0
        fi
    fi
}

# 主函数
main() {
    echo -e "${BLUE}🔍 NUMA 均衡度检查${NC}"
    echo -e "测试标签: ${TEST_LABEL}"
    echo -e "容忍度: ${TOLERANCE}"
    echo ""
    
    check_dependencies
    count_numa_distribution "$TEST_LABEL"
}

# 如果直接执行脚本
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
