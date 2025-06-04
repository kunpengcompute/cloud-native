#!/bin/bash

cpu_count=$(nproc)
target_cpus="0-$(($cpu_count - 1))"
declare -a changes_needed=()

# 初始化列宽变量
max_path_length=0

# 遍历 kubepods.slice 下的所有子目录
while read dir; do
    cpuset_file="$dir/cpuset.cpus"
    
    # 检查 cpuset.cpus 文件是否存在
    if [ -f "$cpuset_file" ]; then
        current_cpus=$(cat "$cpuset_file")
        
        # 如果当前设置不等于目标设置，则记录需要修改的信息
        if [ "$current_cpus" != "$target_cpus" ]; then
            changes_needed+=("$cpuset_file|$current_cpus|$target_cpus")
            
            # 动态计算路径的最大长度
            path_length=${#cpuset_file}
            if [ "$path_length" -gt "$max_path_length" ]; then
                max_path_length=$path_length
            fi
        fi
    fi
done < <(find /sys/fs/cgroup/cpuset/kubepods.slice -mindepth 1 -maxdepth 3 -type d)

# 如果没有需要修改的地方，直接退出
if [ ${#changes_needed[@]} -eq 0 ]; then
    echo "所有 cpuset.cpus 文件的设置已经是目标值，无需修改。"
    exit 0
fi

# 动态计算表格列宽
column_width=$((max_path_length + 5))  # 为路径列增加一些额外空间
current_cpus_width=15
target_cpus_width=15

# 绘制水平分隔线
print_separator() {
    printf "%-${column_width}s%-${current_cpus_width}s%-${target_cpus_width}s\n" | tr ' ' '-'
}

# 格式化表格内容
format_cell() {
    local content=$1
    local width=$2
    local padding=$((($width - ${#content}) / 2))
    printf "%*s%s%*s" $padding "" "$content" $((width - ${#content} - padding)) ""
}

# 以表格形式展示需要修改的地方
echo -e "需要修改的 cpuset.cpus 文件及其当前设置：\n"
print_separator
printf "|$(format_cell "路径" $column_width)|$(format_cell "当前设置" $current_cpus_width)|$(format_cell "目标设置" $target_cpus_width)|\n"
print_separator
for change in "${changes_needed[@]}"; do
    IFS='|' read -r cpuset_file current_cpus target_cpus <<< "$change"
    printf "|$(format_cell "$cpuset_file" $column_width)|$(format_cell "$current_cpus" $current_cpus_width)|$(format_cell "$target_cpus" $target_cpus_width)|\n"
done
print_separator

# 提示用户确认是否进行修改
while true; do
    read -p "是否确认对上述所有文件进行修改？(y/n): " user_confirm
    case "$user_confirm" in
        [Yy])  # 用户输入 y 或 Y
            echo "开始修改..."
            for change in "${changes_needed[@]}"; do
                IFS='|' read -r cpuset_file current_cpus target_cpus <<< "$change"
                echo "$target_cpus" > "$cpuset_file"
                echo "已修改 $cpuset_file 的内容：$current_cpus -> $target_cpus"
            done
            echo "所有需要修改的地方已成功更新。"
            break
            ;;
        [Nn])  # 用户输入 n 或 N
            echo "用户取消修改，脚本退出。"
            exit 1
            ;;
        *)  # 其他非法输入
            echo "无效输入，请输入 y 或 n。"
            ;;
    esac
done