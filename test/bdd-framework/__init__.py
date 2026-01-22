"""
通用 BDD 测试框架

这是一个通用的行为驱动开发（BDD）测试框架，可用于多个 Kubernetes 相关项目的端到端测试。
框架提供了基础的测试工具和抽象，项目可以基于此框架扩展自己的特定测试功能。
"""

__version__ = "1.0.0"
__author__ = "Kunpeng Cloud Computing Team"
__email__ = "kunpeng@huawei.com"

# 导出核心组件
from .core.base_manager import BaseManager
from .core.cluster_manager import ClusterManager
from .core.container_manager import ContainerManager
from .core.resource_validator import ResourceValidator
from .core.metrics_collector import MetricsCollector

__all__ = [
    "BaseManager",
    "ClusterManager", 
    "ContainerManager",
    "ResourceValidator",
    "MetricsCollector"
]
