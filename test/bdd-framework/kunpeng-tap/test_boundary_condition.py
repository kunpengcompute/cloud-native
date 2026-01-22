"""
kunpeng-tap 边界条件测试
测试极端资源请求和边界情况下的调度行为
每个测试超时: 120秒
"""

import pytest
from pytest_bdd import scenarios

# 显式导入步骤定义
from step_definitions.topology_aware_steps import cleanup_test_resources  # noqa
from step_definitions.topology_aware_steps import *  # noqa

# 加载边界条件测试场景
scenarios('features/boundary_condition_tests.feature')

# 为所有边界条件测试设置超时时间（120秒）
pytestmark = pytest.mark.timeout(120)

# 测试前置和后置处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    """每个测试前后的处理"""
    # 前置处理
    print("\n🚀 开始边界条件测试...")

    yield

    # 后置处理 - 清理测试资源
    print("\n🧹 清理边界条件测试资源...")
    try:
        cleanup_test_resources()
        print("✅ 边界条件测试资源清理完成")
    except Exception as e:
        print(f"⚠️ 边界条件测试资源清理时出错: {e}")

