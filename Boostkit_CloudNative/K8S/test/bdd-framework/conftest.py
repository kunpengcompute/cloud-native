"""
通用 BDD 测试框架的 pytest 配置和 fixtures
"""

import pytest
import os
import logging
from pathlib import Path

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(name)s: %(message)s'
)

@pytest.fixture(scope="session")
def test_config():
    """全局测试配置"""
    return {
        "cluster_config": os.getenv("KUBECONFIG", "~/.kube/config"),
        "namespace": os.getenv("TEST_NAMESPACE", "bdd-test"),
        "test_timeout": int(os.getenv("TEST_TIMEOUT", "300")),
        "cleanup_on_failure": os.getenv("CLEANUP_ON_FAILURE", "true").lower() == "true",
        "metrics_endpoint": os.getenv("METRICS_ENDPOINT", "http://localhost:8080/metrics"),
        "log_level": os.getenv("LOG_LEVEL", "INFO"),
    }

@pytest.fixture(scope="session")
def project_root():
    """获取项目根目录"""
    current_dir = Path(__file__).parent
    # 向上查找项目根目录（包含 go.mod 的目录）
    while current_dir.parent != current_dir:
        if (current_dir / "go.mod").exists():
            return current_dir
        current_dir = current_dir.parent
    raise RuntimeError("Could not find project root directory")

@pytest.fixture(scope="function")
def test_namespace(test_config):
    """创建和清理测试命名空间"""
    namespace = test_config["namespace"]
    # 命名空间的创建和清理将在步骤定义中处理
    yield namespace

@pytest.fixture(autouse=True)
def setup_test_environment(test_config):
    """在每个测试前设置测试环境"""
    # 确保测试报告目录存在
    test_reports_dir = Path("test-reports")
    test_reports_dir.mkdir(exist_ok=True)
    
    # 设置环境变量
    os.environ["PYTEST_CURRENT_TEST"] = "true"
    
    # 设置日志级别
    log_level = test_config.get("log_level", "INFO")
    logging.getLogger().setLevel(getattr(logging, log_level.upper()))
    
    yield
    
    # 测试后清理（如果需要）
    if test_config.get("cleanup_on_failure", True):
        # 清理逻辑将在步骤定义中实现
        pass

@pytest.fixture(scope="session")
def framework_config():
    """BDD 框架配置"""
    return {
        "framework_version": "1.0.0",
        "supported_k8s_versions": ["1.28", "1.29", "1.30"],
        "default_timeout": 300,
        "retry_attempts": 3,
        "retry_delay": 5,
    }
