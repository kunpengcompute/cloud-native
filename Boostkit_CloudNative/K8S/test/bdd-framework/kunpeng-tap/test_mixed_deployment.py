"""
kunpeng-tap 混合部署测试
测试在已有容器占用资源的情况下，新容器的拓扑亲和行为
每个测试超时: 150秒
"""

import pytest
from pytest_bdd import scenarios

# 显式导入步骤定义
from step_definitions.topology_aware_steps import cleanup_test_resources  # noqa
from step_definitions.topology_aware_steps import *  # noqa

# 加载混合部署测试场景
scenarios('features/mixed_deployment_tests.feature')

# 为所有混合部署测试设置超时时间（150秒）
pytestmark = pytest.mark.timeout(150)

# 测试前置和后置处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    """每个测试前后的处理"""
    # 前置处理
    print("\n🚀 开始混合部署测试...")

    yield

    # 后置处理 - 清理测试资源
    print("\n🧹 清理混合部署测试资源...")
    try:
        cleanup_test_resources()
        print("✅ 混合部署测试资源清理完成")
    except Exception as e:
        print(f"⚠️ 混合部署测试资源清理时出错: {e}")

