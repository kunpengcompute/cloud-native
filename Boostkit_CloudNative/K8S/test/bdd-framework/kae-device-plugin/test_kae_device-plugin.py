"""
自定义资源支持演示测试

演示框架如何支持：
- 自定义资源（GPU、FPGA 等）
- 环境变量
- Annotations
- 组合配置
"""

import pytest
from pytest_bdd import scenarios

# 导入通用步骤定义
from step_definitions.common_steps import *

# 加载所有演示场景
scenarios('features/kae-device-plugin.feature')

# Fixtures
@pytest.fixture(scope='function', autouse=True)
def cleanup_after_test():
    """每个测试后自动清理资源"""
    yield
    # 测试完成后清理
    cleanup_test_resources()

