#!/bin/bash
# usege: ./qperf.sh client
# set trace level
set -ex

# set variables for test
SERVER_IP="10.56.217.171"
# Client_IP="10.244.1.17"

CORE_BIND_CMD="qperf"
TEST_MODE="sno-host_device-cpu-bind-qperf-unbind"
LOG_DIR="$TEST_MODE"

mkdir -p "$LOG_DIR"
TEST_REPEAT_END=4
LOOP_REPEAT_END=2
# function for server to run
function server(){
    $CORE_BIND_CMD &
}

# generic function for client tests
function run_test(){
    local test_type=$1
    local msg_size=$2
    local is_loop=$3
    local outpath="$LOG_DIR/$TEST_MODE-$test_type-$msg_size"
    local msg_size_params="-m $msg_size"
    local repeat_end=$TEST_REPEAT_END

    if [[ "$is_loop" == "loop" ]]; then
	outpath="$LOG_DIR/$TEST_MODE-$test_type-loop"
        msg_size_params="-oo msg_size:1:1K:*2"
        repeat_end=$LOOP_REPEAT_END
    fi
    echo "test_type: ${test_type} is_loop: ${is_loop}"

    for i in $(seq 0 $repeat_end); do
        echo "repeat: ${i}"  
        $CORE_BIND_CMD "$SERVER_IP" $msg_size_params -t 60 -vu "${test_type}_lat" > "$outpath"-$i.log 2>&1
        # echo "Command completed successfully, sleeping for 5 seconds"
        sleep 5
    done
}

# function for client
function client(){
    # Run TCP tests
    run_test "tcp" 1448 ""
    run_test "tcp" 0 "loop"

    # Run UDP tests
    run_test "udp" 18 ""
    run_test "udp" 0 "loop"
}

set +ex
kill $(pgrep -x qperf)
set -ex

if [[ "$1" == "server" ]]; then
    server
elif [[ "$1" == "client" ]]; then
    client
elif [[ "$1" == "tcp_1448" ]]; then
    run_test "tcp" 1448 ""
elif [[ "$1" == "tcp_loop" ]]; then
    run_test "tcp" 0 "loop"
elif [[ "$1" == "udp_18" ]]; then
    run_test "udp" 18 ""
elif [[ "$1" == "udp_loop" ]]; then
    run_test "udp" 0 "loop"
else
    echo "Usage: $0 {server|client|tcp_1448|tcp_loop|udp_18|udp_loop}"
fi