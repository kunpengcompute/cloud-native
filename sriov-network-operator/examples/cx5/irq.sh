#!/bin/bash
# irq.sh - 中断绑定CPU管理脚本
# 功能：查询/绑定中断到指定CPU
# 用法：sh irq.sh [check|bind] <pci> [cpurange]
# 示例：sh irq.sh check 0000:3b:00.0          # 查询中断绑定
#       sh irq.sh bind 0000:3b:00.0 0-3,5     # 绑定中断到CPU 0,1,2,3,5

# 将CPU范围字符串解析为数组
parse_cpu_list() {
    local IFS_BAK=$IFS
    IFS=','
    local CPU_RANGE_ARR=$1
    IFS=${IFS_BAK}
    CPU_LIST_ARR=()
    local N=0
    for I in ${CPU_RANGE_ARR[@]}; do
        local START=$(echo $I | awk -F'-' '{print $1}')
        local STOP=$(echo $I | awk -F'-' '{print $NF}')
        for X in $(seq $START $STOP); do
            CPU_LIST_ARR[$N]=$X
            N=$((N+1))
        done
    done
}

# 绑定中断到CPU列表
bind_irq() {
    local PCI=$1
    local IRQ_LIST=$(cat /proc/interrupts | grep "${PCI}" | awk -F':' '{print $1}')
    [ -z "$IRQ_LIST" ] && echo "错误：未找到PCI ${PCI}对应的中断" >&2 && return 1
    
    local I=0
    for IRQ in $IRQ_LIST; do
        # 循环使用CPU列表
        [ $I -ge ${#CPU_LIST_ARR[*]} ] && I=0
        echo "${CPU_LIST_ARR[${I}]} -> ${IRQ}"
        echo ${CPU_LIST_ARR[${I}]} >/proc/irq/$IRQ/smp_affinity_list
        I=$((I+1))
    done
}

# 查询中断绑定信息
check_irq() {
    local PCI=$1
    local IRQ_LIST=$(cat /proc/interrupts | grep "${PCI}" | awk -F':' '{print $1}')
    [ -z "$IRQ_LIST" ] && echo "错误：未找到PCI ${PCI}对应的中断" >&2 && return 1
    
    for IRQ in $IRQ_LIST; do
        cat /proc/irq/$IRQ/smp_affinity_list
    done
}

# 参数解析
CMD=$1
PCI=$2
CPU_RANGE=$3

[ -z "$PCI" ] && echo "用法：sh irq.sh [check|bind] <pci> [cpurange]" >&2 && exit 1

# 绑定模式需要CPU范围
if [ "$CMD" == "bind" ]; then
    [ -z "$CPU_RANGE" ] && echo "错误：bind模式需要指定CPU范围" >&2 && exit 1
    parse_cpu_list "$CPU_RANGE"
    bind_irq "$PCI"
elif [ "$CMD" == "check" ]; then
    check_irq "$PCI"
else
    echo "错误：未知命令 $CMD，请使用 check 或 bind" >&2
    exit 1
fi