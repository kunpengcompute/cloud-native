"""
kunpeng-tap 冒烟测试
快速验证核心功能是否正常工作的关键测试
执行时间: ~5分钟
每个测试超时: 120秒
"""

import pytest
from pytest_bdd import scenarios

# 显式导入步骤定义
# 必须先导入 common_steps，确保框架层的步骤定义被注册
from step_definitions import common_steps  # noqa
from step_definitions.topology_aware_steps import cleanup_test_resources  # noqa
from step_definitions.topology_aware_steps import *  # noqa

# 加载冒烟测试场景
scenarios('features/smoke_tests.feature')

# 为所有冒烟测试设置超时时间（120秒，给 Pod 启动留足时间）
pytestmark = pytest.mark.timeout(120)

# 测试前置和后置处理
@pytest.fixture(scope="function", autouse=True)
def cleanup_after_test():
    """每个测试后自动清理资源"""
    # 测试前不需要做什么
    yield

    # 测试后清理资源
    print("\n🧹 清理测试资源...")
    try:
        cleanup_test_resources()
        print("✅ 资源清理完成")
    except Exception as e:
        print(f"⚠️ 资源清理时出错: {e}")

