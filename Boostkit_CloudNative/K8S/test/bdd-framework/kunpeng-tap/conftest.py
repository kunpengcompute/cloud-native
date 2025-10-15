"""
kunpeng-tap BDD 测试的 pytest 配置
"""

import pytest
import os
import sys
from pathlib import Path

# 添加通用 BDD 框架到 Python 路径（当前文件位于 test/bdd-framework/kunpeng-tap/）
framework_path = Path(__file__).parent.parent  # 指向 test/bdd-framework
sys.path.insert(0, str(framework_path))

# 导入通用框架的配置
from conftest import *  # noqa

# 导入所有步骤定义，确保 pytest-bdd 能够发现它们
from step_definitions import topology_aware_steps  # noqa

@pytest.fixture(scope="session")
def kunpeng_tap_config(test_config):
    """kunpeng-tap 特定配置"""
    kunpeng_config = test_config.copy()
    kunpeng_config.update({
        "container_runtime": os.getenv("CONTAINER_RUNTIME", "containerd"),
        "kunpeng_tap_binary": os.getenv("KUNPENG_TAP_BINARY", "/usr/local/bin/kunpeng-tap"),
        "resource_policy": os.getenv("RESOURCE_POLICY", "topology-aware"),
        "enable_memory_topology": os.getenv("ENABLE_MEMORY_TOPOLOGY", "true").lower() == "true",
        "nri_socket_path": os.getenv("NRI_SOCKET_PATH", "/var/run/nri/nri.sock"),
        "kunpeng_tap_namespace": os.getenv("KUNPENG_TAP_NAMESPACE", "topo-affinity-plugin-system"),
    })
    return kunpeng_config

@pytest.fixture(scope="session")
def topology_aware_config():
    """拓扑感知策略配置"""
    return {
        "policy_name": "topology-aware",
        "numa_nodes": 2,
        "cpus_per_node": 4,
        "memory_per_node": "8Gi",
        "enable_gpu_affinity": True,
    }
