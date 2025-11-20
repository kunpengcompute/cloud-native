#!/bin/bash

# shell script to get libkperf files for kunpeng-perf-monitor
set -ex

verbose=true # output details for command execution by default
get_dependencies=true # get libkperf dependencies by default

# 定义一个函数来处理命令执行和输出控制
run_cmd() {
    if [[ "$verbose" == true ]]; then
        "$@"
    else
        "$@" > /dev/null 2>&1
    fi
}

# clean package cache
clean_package_cache() {
    local pkg_manager="$1"
    echo "Cleaning $pkg_manager cache..."
    if [[ "$pkg_manager" == "apt" ]]; then
        run_cmd apt clean
        run_cmd apt autoremove -y
        # clean apt lists directory to save more space
        rm -rf /var/lib/apt/lists/*
    elif [[ "$pkg_manager" == "yum" ]]; then
        run_cmd yum clean all
        run_cmd yum autoremove -y
        # clean yum cache directory to save more space
        rm -rf /var/cache/yum/*
    fi
    # clean generic temporary files
    rm -rf /tmp/* /var/tmp/*
}

# prerequisites for libkperf:
# make cmake numactl-devel python3 gcc glibc # python3 > python3.6 gcc > 4.8.5 glibc > 2.17
# g++ <= 12
get_libkperf_dependencies() {
    # set pkg manager, "yum" is default 
    pkg_manager=""
    numa_pkg="numactl-devel"
    glibc_pkg="glibc"
    if command -v apt &> /dev/null; then
        pkg_manager="apt"
        numa_pkg="libnuma-dev"
        glibc_pkg="libc6"
        run_cmd apt update -y
    elif command -v yum &> /dev/null; then
        pkg_manager="yum"
    else
        echo "failed to find supported pkg manager: yum or apt"
        exit 1
    fi

    # Install dependencies
    run_cmd $pkg_manager install -y make cmake python3 gcc git $numa_pkg $glibc_pkg

    # remove package cache
    clean_package_cache "$pkg_manager"
}

get_libkperf_ready() {
    # download libkperf dependencies
    if [[ "$get_dependencies" == true ]]; then
        get_libkperf_dependencies
    fi

    local workdir="$1"
    # remove old libkperf files
    if [[ -d "$workdir/pkg/kunpeng-perf-monitor/libkperf" ]]; then
        echo "libkperf files already exist in $workdir/pkg/kunpeng-perf-monitor/libkperf, removing..."
        run_cmd rm -rf "$workdir/pkg/kunpeng-perf-monitor/libkperf"
    fi

    if [[ -d "$workdir/tmp/libkperf" ]]; then
        echo "$workdir/tmp/libkperf already exists, skipping downloading libkperf repo"
    else
        mkdir -p "$workdir/tmp" && cd "$workdir/tmp"
        # Clone libkperf repository
        echo "downloading libkperf repo to $workdir/tmp"
        run_cmd git clone https://gitee.com/openeuler/libkperf.git -b v2.0.0
    fi
    cd "$workdir/tmp/libkperf" && run_cmd git submodule update --init --recursive

    # Build libkperf
    run_cmd bash build.sh go=true
    
    # revise the file 
    sed -i 's|"libkperf/sym"|"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-perf-monitor/libkperf/sym"|g' go/src/libkperf/kperf/kperf.go

    # copy files to workdir/pkg/kunpeng-perf-monitor/
    cp -r go/src/libkperf "$workdir/pkg/kunpeng-perf-monitor/"
}

rm_libkperf_files() {
    local workdir="$1"
    # rm the directories related to libkperf
    rm -rf "$workdir/tmp"
}

if [[ "$1" == "get_libkperf_ready" ]]; then
    get_libkperf_ready "$2"
elif [[ "$1" == "rm_libkperf_files" ]]; then
    rm_libkperf_files "$2"
else 
    echo "Usage: bash $0 {get_libkperf_ready|rm_libkperf_files} <workdir>"
    exit 1
fi