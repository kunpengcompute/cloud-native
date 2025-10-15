"""
kunpeng-tap 故障恢复场景测试
测试 tap 插件、容器、containerd 等组件故障后的恢复能力
每个测试超时: 300秒（5分钟）
"""

import pytest
from pytest_bdd import scenarios

# 显式导入步骤定义
from step_definitions.topology_aware_steps import cleanup_test_resources  # noqa
from step_definitions.topology_aware_steps import *  # noqa

# 加载故障恢复测试场景
scenarios('features/failure_recovery_tests.feature')

# 为所有故障恢复测试设置超时时间（300秒 = 5分钟）
pytestmark = pytest.mark.timeout(300)

# 测试前置和后置处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    """每个测试前后的处理"""
    # 前置处理
    print("\n🚀 开始故障恢复测试...")

    yield

    # 后置处理 - 清理测试资源
    print("\n🧹 清理故障恢复测试资源...")
    try:
        cleanup_test_resources()
        print("✅ 故障恢复测试资源清理完成")
    except Exception as e:
        print(f"⚠️ 故障恢复测试资源清理时出错: {e}")

if __name__ == "__main__":
    # 直接执行时运行测试
    pytest.main([__file__, "-v"])
