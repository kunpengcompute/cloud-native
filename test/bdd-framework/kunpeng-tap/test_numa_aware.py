"""
NUMA-Aware 策略 CPU 亲和性测试

测试 numa-aware 策略下容器的 CPU 亲和性行为
主要测试单 NUMA 和跨 NUMA 跨 SOCKET 场景
"""
import pytest
from pytest_bdd import scenarios

# 导入所有步骤定义
from step_definitions.common_steps import *
from step_definitions.topology_aware_steps import *

# 加载所有场景
scenarios('features/numa_aware_tests.feature')

# 为所有测试设置超时间（120秒，给 Pod 启动留足时间）
pytestmark = pytest.mark.timeout(120)

# 测试前后处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    """每个测试前后的设置和清理"""
    print("\n" + "="*70)
    print("🚀 开始 NUMA-Aware 测试...")
    print("="*70)
    
    yield
    
    print("\n" + "="*70)
    print("🧹 清理测试资源...")
    print("="*70)
    cleanup_test_resources()

