"""
kunpeng-tap 拓扑感知测试运行器

使用 pytest-bdd 运行拓扑感知测试场景
每个测试超时: 120秒
"""

import pytest
from pytest_bdd import scenarios

# 显式导入步骤定义
from step_definitions.topology_aware_steps import *  # noqa

# 加载所有测试场景
# 包含 2N-01 到 2N-18, 4N-01 到 4N-24, BC-01 到 BC-05 共 47 个测试
scenarios('features/topology_aware_tests.feature')

# 为所有拓扑感知测试设置超时时间（120秒）
pytestmark = pytest.mark.timeout(120)

# 测试前置和后置处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    """每个测试前后的处理"""
    # 前置处理
    print("\n🚀 开始拓扑感知测试...")

    yield

    # 后置处理 - 清理测试资源
    print("\n🧹 清理测试资源...")
    try:
        cleanup_test_resources()
        print("✅ 资源清理完成")
    except Exception as e:
        print(f"⚠️ 资源清理时出错: {e}")

# 测试配置
@pytest.fixture(scope="session")
def test_config():
    """测试配置"""
    return {
        'timeout': 300,  # 5分钟超时
        'retry_count': 3,
        'cleanup_on_failure': True
    }

# 错误处理
@pytest.fixture(autouse=True)
def handle_test_errors(request):
    """处理测试错误"""
    def cleanup_on_error():
        if hasattr(request.node, 'rep_call') and request.node.rep_call.failed:
            print(f"\n❌ 测试失败: {request.node.name}")
            try:
                cleanup_test_resources()
                print("🧹 已清理失败测试的资源")
            except Exception as e:
                print(f"⚠️ 清理失败测试资源时出错: {e}")
    
    request.addfinalizer(cleanup_on_error)

# pytest hook - 测试结果报告
@pytest.hookimpl(tryfirst=True, hookwrapper=True)
def pytest_runtest_makereport(item, call):
    """生成测试报告"""
    outcome = yield
    rep = outcome.get_result()
    setattr(item, "rep_" + rep.when, rep)
    
    if rep.when == "call":
        if rep.passed:
            print(f"✅ {item.name} - 通过")
        elif rep.failed:
            print(f"❌ {item.name} - 失败")
        elif rep.skipped:
            print(f"⏭️ {item.name} - 跳过")

if __name__ == "__main__":
    # 直接运行测试
    pytest.main([__file__, "-v", "--tb=short"])
