#!/bin/bash

# kunpeng-tap BDD 测试运行脚本
# 用于运行生成的测试场景

set -e

echo "🎯 kunpeng-tap BDD 测试运行脚本"
echo "================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检查依赖
check_dependencies() {
    echo -e "${BLUE}📦 检查依赖...${NC}"
    
    if ! command -v python3 &> /dev/null; then
        echo -e "${RED}❌ Python3 未安装${NC}"
        exit 1
    fi
    
    if ! python3 -c "import pytest, pytest_bdd, yaml" 2>/dev/null; then
        echo -e "${YELLOW}⚠️  缺少依赖，正在安装...${NC}"
        pip install pytest pytest-bdd pyyaml kubernetes
    fi
    
    echo -e "${GREEN}✅ 依赖检查完成${NC}"
}

# 生成测试场景
generate_scenarios() {
    local config_filter="$1"
    local numa_filter="$2"
    echo -e "${BLUE}🔧 生成测试场景...${NC}"

    if [ ! -f "generate_test_cases.py" ]; then
        echo -e "${RED}❌ 测试生成器不存在${NC}"
        exit 1
    fi

    local cmd="python3 generate_test_cases.py --type all"
    local filter_desc=""

    if [ "$config_filter" != "all" ] && [ -n "$config_filter" ]; then
        cmd="$cmd --config $config_filter"
        filter_desc="$config_filter"
    fi

    if [ "$numa_filter" != "all" ] && [ -n "$numa_filter" ]; then
        cmd="$cmd --numa $numa_filter"
        if [ -n "$filter_desc" ]; then
            filter_desc="$filter_desc-$numa_filter"
        else
            filter_desc="$numa_filter"
        fi
    fi

    if [ -n "$filter_desc" ]; then
        echo -e "${YELLOW}🎯 生成 $filter_desc 的测试场景${NC}"
    else
        echo -e "${YELLOW}🎯 生成所有配置的测试场景${NC}"
    fi

    eval $cmd
    echo -e "${GREEN}✅ 测试场景生成完成${NC}"
}

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help              显示帮助信息"
    echo "  -g, --generate          重新生成测试场景"
    echo "  -t, --type TYPE         运行特定类型的测试 (topology|bulk|all)"
    echo "  -c, --config CONFIG     选择机器配置 (standard|high_perf|all)"
    echo "  -n, --numa NUMA         选择NUMA拓扑 (2numa|4numa|all)"
    echo "  -m, --markers MARKERS   运行带特定标记的测试"
    echo "  -v, --verbose           详细输出"
    echo "  -s, --summary           显示测试用例统计"
    echo "  --dry-run              只显示将要运行的测试，不实际执行"
    echo ""
    echo "示例:"
    echo "  $0 -t topology          # 运行拓扑感知测试"
    echo "  $0 -t bulk              # 运行批量部署测试"
    echo "  $0 -c standard          # 运行标准配置的测试"
    echo "  $0 -c high_perf         # 运行高性能配置的测试"
    echo "  $0 -n 2numa             # 运行2NUMA拓扑的测试"
    echo "  $0 -n 4numa             # 运行4NUMA拓扑的测试"
    echo "  $0 -t topology -c standard  # 运行标准配置的拓扑感知测试"
    echo "  $0 -n 2numa -c standard # 运行标准配置的2NUMA测试"
    echo "  $0 -m small_resource    # 运行小资源测试"
    echo "  $0 -m \"cpu_affinity and 2numa\"  # 运行CPU亲和性和2NUMA测试"
    echo "  $0 --dry-run -t all     # 显示所有测试但不执行"
}

# 显示测试统计
show_summary() {
    echo -e "${BLUE}📊 测试用例统计${NC}"
    echo "================================"
    
    if [ -d "features/generated" ]; then
        echo "生成的测试文件:"
        for file in features/generated/*.feature; do
            if [ -f "$file" ]; then
                scenarios=$(grep -c "Scenario:" "$file" 2>/dev/null || echo "0")
                echo -e "  📄 $(basename "$file"): ${GREEN}${scenarios}${NC} 个场景"
            fi
        done
        
        total_scenarios=$(find features/generated -name "*.feature" -exec grep -c "Scenario:" {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}')
        echo -e "\n📈 总计: ${GREEN}${total_scenarios}${NC} 个测试场景"
    else
        echo -e "${YELLOW}⚠️  未找到生成的测试文件，请先运行 -g 生成测试场景${NC}"
    fi
    
    echo ""
    echo "测试类型分布:"
    echo "  🎯 拓扑感知测试: QoS 类型和拓扑亲和验证"
    echo "  📦 批量部署测试: 大批量、高并发容器部署场景"
}

# 运行测试
run_tests() {
    local test_type="$1"
    local config_type="$2"
    local numa_type="$3"
    local markers="$4"
    local verbose="$5"
    local dry_run="$6"
    
    echo -e "${BLUE}🚀 运行测试...${NC}"
    
    # 构建pytest命令
    local pytest_cmd="pytest"
    local test_files=""
    
    # 根据测试类型、配置和NUMA拓扑选择文件
    local filter_desc=""
    if [ "$config_type" != "all" ]; then
        filter_desc="$config_type"
    fi
    if [ "$numa_type" != "all" ]; then
        if [ -n "$filter_desc" ]; then
            filter_desc="$filter_desc-$numa_type"
        else
            filter_desc="$numa_type"
        fi
    fi

    case "$test_type" in
        "topology")
            test_files="test_topology_aware.py"
            if [ -n "$filter_desc" ]; then
                echo -e "${YELLOW}🎯 运行 $filter_desc 的拓扑感知测试${NC}"
            else
                echo -e "${YELLOW}🎯 运行拓扑感知测试${NC}"
            fi
            ;;
        "bulk")
            test_files="test_bulk_deployment.py"
            if [ -n "$filter_desc" ]; then
                echo -e "${YELLOW}📦 运行 $filter_desc 的批量部署测试${NC}"
            else
                echo -e "${YELLOW}📦 运行批量部署测试${NC}"
            fi
            ;;
        "all")
            test_files="test_topology_aware.py test_bulk_deployment.py"
            if [ -n "$filter_desc" ]; then
                echo -e "${YELLOW}🎯 运行 $filter_desc 的所有测试${NC}"
            else
                echo -e "${YELLOW}🎯 运行所有测试${NC}"
            fi
            ;;
        *)
            echo -e "${RED}❌ 未知的测试类型: $test_type${NC}"
            echo "支持的类型: topology, bulk, all"
            exit 1
            ;;
    esac
    
    # 检查测试文件是否存在
    if [ "$test_type" != "all" ] && [ ! -f "$test_files" ]; then
        echo -e "${RED}❌ 测试文件不存在: $test_files${NC}"
        echo -e "${YELLOW}💡 请先运行 $0 -g 生成测试场景${NC}"
        exit 1
    fi
    
    # 添加配置和NUMA过滤
    if [ "$config_type" != "all" ]; then
        if [ -n "$markers" ]; then
            markers="$markers and $config_type"
        else
            markers="$config_type"
        fi
        echo -e "${YELLOW}⚙️  配置过滤: $config_type${NC}"
    fi

    if [ "$numa_type" != "all" ]; then
        if [ -n "$markers" ]; then
            markers="$markers and $numa_type"
        else
            markers="$numa_type"
        fi
        echo -e "${YELLOW}🔢 NUMA过滤: $numa_type${NC}"
    fi

    # 添加标记过滤
    if [ -n "$markers" ]; then
        pytest_cmd="$pytest_cmd -m \"$markers\""
        echo -e "${YELLOW}🏷️  使用标记过滤: $markers${NC}"
    fi
    
    # 添加详细输出
    if [ "$verbose" = "true" ]; then
        pytest_cmd="$pytest_cmd -v"
    fi
    
    # 添加测试文件
    pytest_cmd="$pytest_cmd $test_files"
    
    # 干运行模式
    if [ "$dry_run" = "true" ]; then
        echo -e "${YELLOW}🔍 干运行模式 - 显示将要运行的测试:${NC}"
        pytest_cmd="$pytest_cmd --collect-only -q"
    fi
    
    echo -e "${BLUE}执行命令: $pytest_cmd${NC}"
    echo ""
    
    # 执行测试
    eval $pytest_cmd
    
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        echo -e "\n${GREEN}✅ 测试完成${NC}"
    else
        echo -e "\n${RED}❌ 测试失败 (退出码: $exit_code)${NC}"
    fi
    
    return $exit_code
}

# 主函数
main() {
    local generate_flag=false
    local test_type="all"
    local config_type="all"
    local numa_type="all"
    local markers=""
    local verbose=false
    local dry_run=false
    local show_summary_flag=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -g|--generate)
                generate_flag=true
                shift
                ;;
            -t|--type)
                test_type="$2"
                shift 2
                ;;
            -c|--config)
                config_type="$2"
                shift 2
                ;;
            -n|--numa)
                numa_type="$2"
                shift 2
                ;;
            -m|--markers)
                markers="$2"
                shift 2
                ;;
            -v|--verbose)
                verbose=true
                shift
                ;;
            -s|--summary)
                show_summary_flag=true
                shift
                ;;
            --dry-run)
                dry_run=true
                shift
                ;;
            *)
                echo -e "${RED}❌ 未知选项: $1${NC}"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 检查依赖
    check_dependencies
    
    # 显示统计信息
    if [ "$show_summary_flag" = "true" ]; then
        show_summary
        exit 0
    fi
    
    # 生成测试场景
    if [ "$generate_flag" = "true" ]; then
        generate_scenarios "$config_type" "$numa_type"
    fi

    # 检查是否有生成的测试文件
    if [ ! -d "features/generated" ] || [ -z "$(ls -A features/generated 2>/dev/null)" ]; then
        echo -e "${YELLOW}⚠️  未找到生成的测试文件${NC}"
        echo -e "${BLUE}💡 正在自动生成测试场景...${NC}"
        generate_scenarios "$config_type" "$numa_type"
    fi

    # 运行测试
    run_tests "$test_type" "$config_type" "$numa_type" "$markers" "$verbose" "$dry_run"
}

# 如果直接执行脚本
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
