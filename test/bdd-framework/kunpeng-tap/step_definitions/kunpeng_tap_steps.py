"""
kunpeng-tap 特定的 BDD 步骤定义
"""

import pytest
import logging
from pytest_bdd import given, when, then, parsers

# 导入通用框架组件
from bdd_framework.core.cluster_manager import ClusterManager
from bdd_framework.core.container_manager import ContainerManager
from bdd_framework.core.resource_validator import ResourceValidator

# 导入 kunpeng-tap 特定组件
from ..support.kunpeng_tap_manager import KunpengTapManager
from ..support.numa_analyzer import NumaAnalyzer

logger = logging.getLogger(__name__)

@given("kunpeng-tap plugin is installed and running")
def kunpeng_tap_is_running(kunpeng_tap_config):
    """验证 kunpeng-tap 插件已安装并运行"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    
    # 检查二进制文件
    assert tap_manager.is_binary_available(), "kunpeng-tap binary is not available"
    
    # 检查服务状态
    assert tap_manager.is_service_running(), "kunpeng-tap service is not running"
    
    logger.info("kunpeng-tap plugin is installed and running")

@given("the topology-aware policy is enabled")
def topology_aware_policy_enabled(kunpeng_tap_config):
    """验证拓扑感知策略已启用"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    policy_config = tap_manager.get_policy_configuration()
    
    assert policy_config.get("resource_policy") == "topology-aware", \
        f"Expected topology-aware policy, got: {policy_config.get('resource_policy')}"
    
    logger.info("Topology-aware policy is enabled")

@given("containerd is configured as the container runtime")
def containerd_is_configured(kunpeng_tap_config):
    """验证 containerd 已配置为容器运行时"""
    cluster_manager = ClusterManager(kunpeng_tap_config)
    runtime_info = cluster_manager.get_container_runtime_info()
    
    assert "containerd" in runtime_info.lower(), f"Expected containerd, got: {runtime_info}"
    logger.info("Containerd is configured as the container runtime")

@given("kunpeng-tap is configured for containerd integration")
def kunpeng_tap_containerd_configured(kunpeng_tap_config):
    """验证 kunpeng-tap 已配置为与 containerd 集成"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    config_info = tap_manager.get_containerd_configuration()
    
    assert config_info.get("container_runtime_mode") == "NRI", \
        "kunpeng-tap is not configured for NRI mode"
    
    logger.info("kunpeng-tap is configured for containerd integration")

@when(parsers.parse("I deploy a CPU-intensive container with resource requests:\n{resource_table}"))
def deploy_cpu_intensive_container(resource_table, test_namespace, kunpeng_tap_config):
    """部署 CPU 密集型容器"""
    container_manager = ContainerManager(kunpeng_tap_config)
    
    # 解析资源表
    resources = container_manager.parse_resource_table(resource_table)
    
    # 创建容器规范
    container_spec = {
        "name": "cpu-intensive-test",
        "image": "stress:latest",
        "image_pull_policy": "IfNotPresent",  # 优先使用本地镜像
        "resources": resources,
        "command": ["stress", "--cpu", "2", "--timeout", "300s"],
        "annotations": {
            "kunpeng.huawei.com/topology-aware": "true"
        }
    }
    
    # 部署容器
    pod_name = container_manager.deploy_pod(test_namespace, container_spec)
    
    # 存储 Pod 名称供后续验证
    pytest.current_test_context = getattr(pytest, 'current_test_context', {})
    pytest.current_test_context['deployed_pods'] = [pod_name]
    
    logger.info(f"Deployed CPU-intensive container: {pod_name}")

@then("the container should be assigned to a single NUMA node")
def container_assigned_single_numa_node(kunpeng_tap_config):
    """验证容器被分配到单个 NUMA 节点"""
    numa_analyzer = NumaAnalyzer(kunpeng_tap_config)
    pytest.current_test_context = getattr(pytest, 'current_test_context', {})
    deployed_pods = pytest.current_test_context.get('deployed_pods', [])
    
    assert len(deployed_pods) > 0, "No deployed pods found"
    
    for pod_name in deployed_pods:
        numa_assignment = numa_analyzer.get_pod_numa_assignment(pod_name)
        assert len(numa_assignment) == 1, f"Pod {pod_name} is assigned to multiple NUMA nodes: {numa_assignment}"
    
    logger.info("All containers are assigned to single NUMA nodes")

@then("the CPU affinity should match the assigned NUMA node")
def cpu_affinity_matches_numa_node(kunpeng_tap_config):
    """验证 CPU 亲和性匹配分配的 NUMA 节点"""
    numa_analyzer = NumaAnalyzer(kunpeng_tap_config)
    pytest.current_test_context = getattr(pytest, 'current_test_context', {})
    deployed_pods = pytest.current_test_context.get('deployed_pods', [])
    
    for pod_name in deployed_pods:
        cpu_affinity = numa_analyzer.get_pod_cpu_affinity(pod_name)
        numa_assignment = numa_analyzer.get_pod_numa_assignment(pod_name)
        
        # 验证 CPU 亲和性匹配 NUMA 节点
        expected_cpus = numa_analyzer.get_numa_node_cpus(numa_assignment[0])
        assert cpu_affinity.issubset(expected_cpus), \
            f"CPU affinity {cpu_affinity} doesn't match NUMA node {numa_assignment[0]} CPUs {expected_cpus}"
    
    logger.info("CPU affinity matches assigned NUMA nodes")

@then("the memory should be allocated from the same NUMA node")
def memory_allocated_same_numa_node(kunpeng_tap_config):
    """验证内存从相同的 NUMA 节点分配"""
    numa_analyzer = NumaAnalyzer(kunpeng_tap_config)
    pytest.current_test_context = getattr(pytest, 'current_test_context', {})
    deployed_pods = pytest.current_test_context.get('deployed_pods', [])
    
    for pod_name in deployed_pods:
        memory_nodes = numa_analyzer.get_pod_memory_nodes(pod_name)
        numa_assignment = numa_analyzer.get_pod_numa_assignment(pod_name)
        
        assert memory_nodes == numa_assignment, \
            f"Memory nodes {memory_nodes} don't match NUMA assignment {numa_assignment}"
    
    logger.info("Memory is allocated from the same NUMA nodes")

@then("the container should be scheduled successfully")
def container_scheduled_successfully(kunpeng_tap_config):
    """验证容器成功调度"""
    validator = ResourceValidator(kunpeng_tap_config)
    pytest.current_test_context = getattr(pytest, 'current_test_context', {})
    deployed_pods = pytest.current_test_context.get('deployed_pods', [])
    
    assert len(deployed_pods) > 0, "No deployed pods found"
    
    namespace = kunpeng_tap_config.get('namespace', 'default')
    for pod_name in deployed_pods:
        assert validator.validate_pod_status(namespace, pod_name, "Running"), \
            f"Pod {pod_name} is not running"
    
    logger.info("All containers are scheduled successfully")

@when("kunpeng-tap is started in NRI mode")
def kunpeng_tap_started_nri_mode(kunpeng_tap_config):
    """启动 kunpeng-tap NRI 模式"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    
    # 确保服务以 NRI 模式运行
    if not tap_manager.is_service_running():
        success = tap_manager.start_service("NRI")
        assert success, "Failed to start kunpeng-tap in NRI mode"
    
    logger.info("kunpeng-tap started in NRI mode")

@then("kunpeng-tap should be registered as an NRI plugin")
def kunpeng_tap_registered_nri_plugin(kunpeng_tap_config):
    """验证 kunpeng-tap 已注册为 NRI 插件"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    
    assert tap_manager.check_nri_registration(), \
        "kunpeng-tap is not registered as NRI plugin"
    
    logger.info("kunpeng-tap is registered as NRI plugin")

@then("the plugin should be active and responding")
def plugin_active_and_responding(kunpeng_tap_config):
    """验证插件处于活动状态并响应"""
    tap_manager = KunpengTapManager(kunpeng_tap_config)
    
    assert tap_manager.is_service_running(), "kunpeng-tap service is not running"
    
    # 检查服务日志中的活动迹象
    logs = tap_manager.get_service_logs(20)
    assert "plugin" in logs.lower() or "nri" in logs.lower(), \
        "No plugin activity found in logs"
    
    logger.info("Plugin is active and responding")
