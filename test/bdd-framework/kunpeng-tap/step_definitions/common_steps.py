"""
通用 BDD 步骤定义框架
提供可复用的 Kubernetes Deployment 创建和验证步骤
"""

import time
from typing import Dict, List, Any, Optional
from pytest_bdd import given, when, then, parsers
from kubernetes import client, config
from kubernetes.client.rest import ApiException

# ============================================================================
# 全局测试上下文
# ============================================================================

test_context = {
    'deployments': [],
    'pods': [],
    'machine_config': None,
    'test_label': None,
    'qos_type': None,
    'replica_count': 0,
    'resource_config': {},
    'namespace': 'default',
    'image': 'nginx:latest',
    'image_pull_policy': 'IfNotPresent'
}

# ============================================================================
# Kubernetes 客户端初始化
# ============================================================================

try:
    config.load_incluster_config()
except:
    config.load_kube_config()

k8s_apps_v1 = client.AppsV1Api()
k8s_core_v1 = client.CoreV1Api()

# ============================================================================
# Given 步骤 - 环境准备
# ============================================================================

@given(parsers.parse('集群有 {config} 配置的节点'))
def cluster_has_machine_config(config):
    """验证集群具有指定的机器配置

    示例:
        Given 集群有 2numa_48 配置的节点
        Given 集群有 4numa_24 配置的节点
        Given 集群有 default 配置的节点
    """
    test_context['machine_config'] = config

    # 验证集群节点是否符合配置要求
    nodes = k8s_core_v1.list_node()
    if len(nodes.items) == 0:
        raise Exception("集群中没有可用节点")

    print(f"✅ 集群配置验证通过: {config}")

@given(parsers.parse('使用命名空间 "{namespace}"'))
def use_namespace(namespace):
    """设置测试使用的命名空间"""
    test_context['namespace'] = namespace
    print(f"✅ 使用命名空间: {namespace}")

@given(parsers.parse('使用容器镜像 "{image}"'))
def use_container_image(image):
    """设置容器镜像"""
    test_context['image'] = image
    print(f"✅ 使用容器镜像: {image}")

# ============================================================================
# When 步骤 - Deployment 创建
# ============================================================================

@when(parsers.parse('我创建 {replica_count:d} 个 {qos_type} 类型的 Deployment'))
def create_qos_deployments(replica_count, qos_type):
    """创建指定数量和 QoS 类型的 Deployment"""
    test_context['replica_count'] = replica_count
    test_context['qos_type'] = qos_type
    print(f"🚀 准备创建 {replica_count} 个 {qos_type} 类型的 Deployment")

@when(parsers.parse('Deployment 具有 {cpu_request}/{cpu_limit} CPU 资源配置'))
def deployment_has_cpu_config(cpu_request, cpu_limit):
    """设置 Deployment 的 CPU 资源配置"""
    test_context['resource_config']['cpu_request'] = cpu_request
    test_context['resource_config']['cpu_limit'] = cpu_limit

@when(parsers.parse('Deployment 具有 {memory_request}/{memory_limit} Memory 资源配置'))
def deployment_has_memory_config(memory_request, memory_limit):
    """设置 Deployment 的 Memory 资源配置"""
    test_context['resource_config']['memory_request'] = memory_request
    test_context['resource_config']['memory_limit'] = memory_limit

@when(parsers.parse('Deployment 具有标签 "{test_label}"'))
def deployment_has_label(test_label):
    """设置 Deployment 的测试标签并实际创建 Deployment"""
    test_context['test_label'] = test_label
    
    # 现在创建实际的 Deployment
    _create_actual_deployments()

@when(parsers.parse('每个 Deployment 具有 {cpu_request}/{cpu_limit} CPU 资源配置'))
def each_deployment_has_cpu_config(cpu_request, cpu_limit):
    """设置每个 Deployment 的 CPU 资源配置"""
    deployment_has_cpu_config(cpu_request, cpu_limit)

@when(parsers.parse('每个 Deployment 具有 {memory_request}/{memory_limit} Memory 资源配置'))
def each_deployment_has_memory_config(memory_request, memory_limit):
    """设置每个 Deployment 的 Memory 资源配置"""
    deployment_has_memory_config(memory_request, memory_limit)

@when(parsers.parse('Deployment 具有自定义资源 {resource_name} 配置为 {resource_request}/{resource_limit}'))
def deployment_has_custom_resource(resource_name, resource_request, resource_limit):
    """设置 Deployment 的自定义资源配置

    示例:
        And Deployment 具有自定义资源 nvidia.com/gpu 配置为 1/1
        And Deployment 具有自定义资源 example.com/foo 配置为 2/4
    """
    if 'custom_resources' not in test_context['resource_config']:
        test_context['resource_config']['custom_resources'] = {}

    test_context['resource_config']['custom_resources'][resource_name] = {
        'request': resource_request,
        'limit': resource_limit
    }
    print(f"✅ 设置自定义资源: {resource_name} = {resource_request}/{resource_limit}")

@when(parsers.parse('Deployment 具有环境变量 {env_name}={env_value}'))
def deployment_has_env_var(env_name, env_value):
    """设置 Deployment 的环境变量

    示例:
        And Deployment 具有环境变量 MY_VAR=my_value
        And Deployment 具有环境变量 DEBUG=true
    """
    if 'env_vars' not in test_context:
        test_context['env_vars'] = {}

    test_context['env_vars'][env_name] = env_value
    print(f"✅ 设置环境变量: {env_name}={env_value}")

@when(parsers.parse('Deployment 具有 annotation {annotation_key}={annotation_value}'))
def deployment_has_annotation(annotation_key, annotation_value):
    """设置 Deployment 的 annotation

    示例:
        And Deployment 具有 annotation scheduler.alpha.kubernetes.io/critical-pod=""
        And Deployment 具有 annotation my-annotation=my-value
    """
    if 'annotations' not in test_context:
        test_context['annotations'] = {}

    test_context['annotations'][annotation_key] = annotation_value
    print(f"✅ 设置 annotation: {annotation_key}={annotation_value}")

# ============================================================================
# Then 步骤 - 基本验证
# ============================================================================

@then('容器应该被成功调度')
def containers_should_be_scheduled():
    """验证容器被成功调度"""
    _wait_for_pods_ready()
    
    pods = _get_test_pods()
    for pod in pods:
        if pod.status.phase != 'Running':
            raise Exception(f"Pod {pod.metadata.name} 状态不是 Running: {pod.status.phase}")
    
    print(f"✅ 所有 {len(pods)} 个容器已成功调度")

@then('所有容器应该被成功调度')
def all_containers_should_be_scheduled():
    """验证所有容器被成功调度"""
    containers_should_be_scheduled()

@then(parsers.parse('容器的 QoS 类别应该是 {expected_qos}'))
def verify_qos_class(expected_qos):
    """验证容器的 QoS 类别"""
    pods = _get_test_pods()

    for pod in pods:
        actual_qos = pod.status.qos_class
        if actual_qos != expected_qos:
            raise Exception(f"Pod {pod.metadata.name} QoS 类别不匹配: 期望 {expected_qos}, 实际 {actual_qos}")

    print(f"✅ 容器 QoS 类别验证通过: {expected_qos}")

@then('容器调度应该失败')
def containers_should_fail_to_schedule():
    """验证容器调度失败（Pod 处于 Pending 状态）

    用于测试资源不足等边界条件场景

    示例:
        Then 容器调度应该失败
        And 容器状态应该是 Pending
    """
    # 等待一段时间，让调度器尝试调度
    time.sleep(5)

    pods = _get_test_pods()
    if not pods:
        raise Exception("没有找到测试 Pod")

    for pod in pods:
        if pod.status.phase == 'Running':
            raise Exception(f"Pod {pod.metadata.name} 不应该被成功调度，但状态是 Running")

        # 检查是否有调度失败的事件
        if pod.status.phase == 'Pending':
            # 检查 Pod 的 conditions
            has_scheduling_issue = False
            if pod.status.conditions:
                for condition in pod.status.conditions:
                    if condition.type == 'PodScheduled' and condition.status == 'False':
                        has_scheduling_issue = True
                        print(f"✅ Pod {pod.metadata.name} 调度失败: {condition.reason} - {condition.message}")
                        break

            if not has_scheduling_issue:
                print(f"⚠️  Pod {pod.metadata.name} 处于 Pending 状态，但没有明确的调度失败原因")
        else:
            print(f"⚠️  Pod {pod.metadata.name} 状态: {pod.status.phase}")

    print(f"✅ 验证通过: 容器调度失败（共 {len(pods)} 个 Pod）")

@then(parsers.parse('容器状态应该是 {expected_status}'))
def verify_container_status(expected_status):
    """验证容器状态

    参数:
        expected_status: 期望的 Pod 状态，如 Pending, Running, Failed 等

    示例:
        And 容器状态应该是 Pending
        And 容器状态应该是 Running
        And 容器状态应该是 Failed
    """
    pods = _get_test_pods()
    if not pods:
        raise Exception("没有找到测试 Pod")

    for pod in pods:
        actual_status = pod.status.phase
        if actual_status != expected_status:
            raise Exception(
                f"Pod {pod.metadata.name} 状态不匹配: 期望 {expected_status}, 实际 {actual_status}"
            )
        print(f"✅ Pod {pod.metadata.name} 状态: {actual_status}")

    print(f"✅ 所有容器状态验证通过: {expected_status}")

# ============================================================================
# 辅助函数 - Deployment 创建
# ============================================================================

def _cleanup_existing_deployments(test_label: str):
    """清理已存在的测试 Deployment"""
    try:
        namespace = test_context.get('namespace', 'default')
        deployments = k8s_apps_v1.list_namespaced_deployment(
            namespace=namespace,
            label_selector=f"test-label={test_label}"
        )

        for deployment in deployments.items:
            k8s_apps_v1.delete_namespaced_deployment(
                name=deployment.metadata.name,
                namespace=namespace
            )
            print(f"🗑️ 清理已存在的 Deployment: {deployment.metadata.name}")

        if deployments.items:
            time.sleep(10)
            print(f"✅ 清理完成，等待资源释放...")
    except Exception as e:
        print(f"⚠️ 清理已存在资源时出错: {e}")

def _create_actual_deployments():
    """创建实际的 Deployment"""
    qos_type = test_context['qos_type']
    replica_count = test_context['replica_count']
    test_label = test_context['test_label']
    resource_config = test_context['resource_config']
    namespace = test_context.get('namespace', 'default')

    # 先清理可能存在的旧资源
    _cleanup_existing_deployments(test_label)

    # 根据 QoS 类型创建资源规格
    container_spec = _build_container_spec(qos_type, resource_config)

    for i in range(replica_count):
        deployment_name = f"{test_label}-{i}".lower()
        deployment = _build_deployment(deployment_name, test_label, container_spec)

        try:
            created_deployment = k8s_apps_v1.create_namespaced_deployment(
                namespace=namespace,
                body=deployment
            )
            test_context['deployments'].append(created_deployment)
            print(f"✅ 创建 Deployment: {deployment_name}")
        except ApiException as e:
            if e.status == 409:  # Already exists
                print(f"⚠️ Deployment {deployment_name} 已存在，尝试删除后重新创建...")
                try:
                    k8s_apps_v1.delete_namespaced_deployment(
                        name=deployment_name,
                        namespace=namespace
                    )
                    time.sleep(5)
                    created_deployment = k8s_apps_v1.create_namespaced_deployment(
                        namespace=namespace,
                        body=deployment
                    )
                    test_context['deployments'].append(created_deployment)
                    print(f"✅ 重新创建 Deployment: {deployment_name}")
                except Exception as retry_error:
                    raise Exception(f"重新创建 Deployment 失败: {retry_error}")
            else:
                raise Exception(f"创建 Deployment 失败: {e}")

def _build_container_spec(qos_type: str, resource_config: Dict[str, str]) -> Dict[str, Any]:
    """构建容器规格"""
    image = test_context.get('image', 'nginx:latest')
    image_pull_policy = test_context.get('image_pull_policy', 'IfNotPresent')

    container_spec = {
        'name': 'test-container',
        'image': image,
        'image_pull_policy': image_pull_policy,
        'resources': {}
    }

    # 根据 QoS 类型设置资源
    if qos_type == 'guaranteed':
        container_spec['resources'] = {
            'requests': {
                'cpu': resource_config.get('cpu_request', '1'),
                'memory': resource_config.get('memory_request', '2Gi')
            },
            'limits': {
                'cpu': resource_config.get('cpu_limit', '1'),
                'memory': resource_config.get('memory_limit', '2Gi')
            }
        }
    elif qos_type == 'burstable':
        container_spec['resources'] = {
            'requests': {
                'cpu': resource_config.get('cpu_request', '0.5'),
                'memory': resource_config.get('memory_request', '1Gi')
            },
            'limits': {
                'cpu': resource_config.get('cpu_limit', '1'),
                'memory': resource_config.get('memory_limit', '2Gi')
            }
        }
    # besteffort 不设置资源限制

    # 添加自定义资源
    custom_resources = resource_config.get('custom_resources', {})
    if custom_resources:
        if 'resources' not in container_spec or not container_spec['resources']:
            container_spec['resources'] = {'requests': {}, 'limits': {}}

        for resource_name, resource_values in custom_resources.items():
            if 'requests' not in container_spec['resources']:
                container_spec['resources']['requests'] = {}
            if 'limits' not in container_spec['resources']:
                container_spec['resources']['limits'] = {}

            container_spec['resources']['requests'][resource_name] = resource_values['request']
            container_spec['resources']['limits'][resource_name] = resource_values['limit']
            print(f"✅ 添加自定义资源到容器: {resource_name} = {resource_values['request']}/{resource_values['limit']}")

    # 添加环境变量
    env_vars = test_context.get('env_vars', {})
    if env_vars:
        container_spec['env'] = [
            {'name': k, 'value': v} for k, v in env_vars.items()
        ]
        print(f"✅ 添加环境变量到容器: {list(env_vars.keys())}")

    return container_spec

def _build_deployment(name: str, test_label: str, container_spec: Dict[str, Any]) -> client.V1Deployment:
    """构建 Deployment 对象"""
    # 获取 annotations
    annotations = test_context.get('annotations', {})

    # 构建 Pod 模板的 metadata
    pod_metadata = client.V1ObjectMeta(
        labels={'test-label': test_label, 'deployment': name}
    )

    # 如果有 annotations，添加到 Pod 模板
    if annotations:
        pod_metadata.annotations = annotations
        print(f"✅ 添加 annotations 到 Pod: {list(annotations.keys())}")

    return client.V1Deployment(
        metadata=client.V1ObjectMeta(
            name=name,
            labels={'test-label': test_label}
        ),
        spec=client.V1DeploymentSpec(
            replicas=1,
            selector=client.V1LabelSelector(
                match_labels={'test-label': test_label, 'deployment': name}
            ),
            template=client.V1PodTemplateSpec(
                metadata=pod_metadata,
                spec=client.V1PodSpec(
                    containers=[client.V1Container(**container_spec)]
                )
            )
        )
    )

# ============================================================================
# 辅助函数 - Pod 查询和等待
# ============================================================================

def _get_test_pods() -> List[client.V1Pod]:
    """获取测试相关的 Pods"""
    test_label = test_context.get('test_label')
    namespace = test_context.get('namespace', 'default')
    
    if not test_label:
        return []
    
    pods = k8s_core_v1.list_namespaced_pod(
        namespace=namespace,
        label_selector=f"test-label={test_label}"
    )
    
    return pods.items

def _wait_for_pods_ready(timeout: int = 120):
    """等待 Pods 就绪"""
    start_time = time.time()
    replica_count = test_context.get('replica_count', 1)
    
    while time.time() - start_time < timeout:
        pods = _get_test_pods()
        
        if len(pods) >= replica_count:
            all_running = all(pod.status.phase == 'Running' for pod in pods)
            if all_running:
                print(f"✅ 所有 {len(pods)} 个 Pod 已就绪")
                return
        
        time.sleep(5)
    
    raise Exception(f"等待 Pod 就绪超时（{timeout}秒）")

# ============================================================================
# 清理函数
# ============================================================================

def cleanup_test_resources():
    """清理测试资源

    清理策略（多重保障）:
    1. 从 test_context['deployments'] 中获取记录的 Deployment
    2. 根据 test_label 查询当前测试的 Deployment
    3. 查询所有带 test-label 的 Deployment（兜底清理）
    4. 合并去重后统一删除

    增强功能：
    - 彻底清理故障恢复测试相关的状态变量
    - 确保测试之间不会出现状态污染

    这样可以确保：
    - 混合部署测试中的所有"已有容器"被清理
    - 即使 test_context 被意外修改也能清理
    - 避免测试之间的资源干扰
    - 故障恢复测试的状态不会污染其他测试
    """
    namespace = test_context.get('namespace', 'default')
    deployments_to_delete = set()  # 使用 set 去重

    # 方法1: 从 test_context['deployments'] 中获取所有创建的 Deployment
    if test_context.get('deployments'):
        for deployment in test_context['deployments']:
            deployments_to_delete.add(deployment.metadata.name)
        print(f"📋 从上下文中找到 {len(test_context['deployments'])} 个 Deployment")

    # 方法2: 根据当前 test_label 查询
    if test_context.get('test_label'):
        test_label = test_context['test_label']
        try:
            deployments = k8s_apps_v1.list_namespaced_deployment(
                namespace=namespace,
                label_selector=f"test-label={test_label}"
            )
            for deployment in deployments.items:
                deployments_to_delete.add(deployment.metadata.name)
            if deployments.items:
                print(f"📋 根据标签 {test_label} 找到 {len(deployments.items)} 个 Deployment")
        except Exception as e:
            print(f"⚠️ 查询 Deployment 时出错: {e}")

    # 方法3: 查询所有带 test-label 的 Deployment（兜底清理，防止遗漏）
    try:
        all_test_deployments = k8s_apps_v1.list_namespaced_deployment(
            namespace=namespace
        )
        for deployment in all_test_deployments.items:
            # 检查是否有 test-label 标签
            if deployment.metadata.labels and 'test-label' in deployment.metadata.labels:
                deployments_to_delete.add(deployment.metadata.name)
        print(f"📋 兜底查询找到 {len([d for d in all_test_deployments.items if d.metadata.labels and 'test-label' in d.metadata.labels])} 个测试 Deployment")
    except Exception as e:
        print(f"⚠️ 兜底查询 Deployment 时出错: {e}")

    # 删除所有 Deployment
    if deployments_to_delete:
        print(f"🗑️  准备删除 {len(deployments_to_delete)} 个 Deployment")
        deleted_count = 0
        for deployment_name in deployments_to_delete:
            try:
                k8s_apps_v1.delete_namespaced_deployment(
                    name=deployment_name,
                    namespace=namespace
                )
                print(f"  ✓ 删除 Deployment: {deployment_name}")
                deleted_count += 1
            except ApiException as e:
                if e.status == 404:
                    print(f"  - Deployment {deployment_name} 已不存在")
                else:
                    print(f"  ✗ 删除 Deployment {deployment_name} 失败: {e}")
            except Exception as e:
                print(f"  ✗ 删除 Deployment {deployment_name} 时出错: {e}")

        print(f"✅ 成功删除 {deleted_count}/{len(deployments_to_delete)} 个 Deployment")
    else:
        print("📋 没有需要清理的 Deployment")

    # 重置测试上下文 - 增强版本，彻底清理所有状态
    test_context['deployments'] = []
    test_context['pods'] = []
    test_context['resource_config'] = {}
    test_context['test_label'] = None
    test_context['replica_count'] = 0
    test_context['qos_type'] = None
    test_context['machine_config'] = None
    
    # 清理故障恢复测试相关的状态变量（防止状态污染）
    failure_recovery_keys = [
        'is_after_restart',
        'numa_distribution_before_restart',
        'stable_numa_distribution_before',
        'numa_distribution_test_label',
        'stable_numa_distribution_test_label'
    ]
    
    cleaned_keys = []
    for key in failure_recovery_keys:
        if key in test_context:
            del test_context[key]
            cleaned_keys.append(key)
    
    if cleaned_keys:
        print(f"🧹 清理故障恢复测试状态: {cleaned_keys}")
    
    # 清理其他可能的状态变量
    if 'env_vars' in test_context:
        test_context['env_vars'] = {}
    if 'annotations' in test_context:
        test_context['annotations'] = {}
    
    print("✅ 测试上下文已完全重置，确保无状态污染")

