"""
Common, reusable Kubernetes BDD step definitions.

Goals:
- Provide minimal, generic steps to create Deployments/Pods with resources
- Verify scheduling success and QoS class
- Keep no project-specific logic (e.g., topology plugin checks)

Usage:
- In your project's step_definitions, import these steps:
    from bdd_framework.common_step_defs.common_k8s_steps import *  # noqa
- Ensure pytest can locate Kubernetes config (in-cluster or KUBECONFIG)
- Ensure you have the session/function fixtures from bdd-framework/conftest.py
"""

from __future__ import annotations

import os
import time
from typing import Dict, List, Any

from pytest_bdd import given, when, then, parsers

try:
    from kubernetes import client, config
    from kubernetes.client.rest import ApiException
except Exception:  # pragma: no cover
    client = None  # type: ignore
    config = None  # type: ignore
    ApiException = Exception  # type: ignore

# ----------------------------------------------------------------------------
# Global test context (kept intentionally simple and generic)
# ----------------------------------------------------------------------------

test_context: Dict[str, Any] = {
    "deployments": [],
    "pods": [],
    "test_label": None,
    "replica_count": 0,
    "resource_config": {},
}

# ----------------------------------------------------------------------------
# Kubernetes client bootstrap (works both in-cluster and out-of-cluster)
# ----------------------------------------------------------------------------

def _ensure_k8s():
    if client is None:
        raise RuntimeError("kubernetes package is not available. Please install it first.")
    try:
        config.load_incluster_config()
    except Exception:
        config.load_kube_config()

_apps: client.AppsV1Api | None = None
_core: client.CoreV1Api | None = None


def _apps_api() -> client.AppsV1Api:
    global _apps
    if _apps is None:
        _ensure_k8s()
        _apps = client.AppsV1Api()
    return _apps


def _core_api() -> client.CoreV1Api:
    global _core
    if _core is None:
        _ensure_k8s()
        _core = client.CoreV1Api()
    return _core

# ----------------------------------------------------------------------------
# Given steps (cluster readiness)
# ----------------------------------------------------------------------------

def _is_node_ready(node) -> bool:
    """Check if a node is in Ready state."""
    if not node.status or not node.status.conditions:
        return False
    for condition in node.status.conditions:
        if condition.type == "Ready" and condition.status == "True":
            return True
    return False


def _is_node_schedulable(node) -> bool:
    """Check if a node is schedulable (not cordoned)."""
    return not node.spec.unschedulable if node.spec.unschedulable is not None else True


@given("集群有可用的 Kubernetes 节点")
@given("the Kubernetes cluster has available nodes")
def cluster_has_available_nodes():
    """
    Validate cluster has at least one schedulable node (generic health check).

    Checks:
    1. At least one node exists in the cluster
    2. At least one node is in Ready state
    3. Node is not cordoned (schedulable)
    """
    nodes = _core_api().list_node()
    if len(nodes.items) == 0:
        raise AssertionError("No nodes available in the cluster")

    # Check for Ready and schedulable nodes
    ready_nodes = []
    for node in nodes.items:
        node_name = node.metadata.name
        is_ready = _is_node_ready(node)
        is_schedulable = _is_node_schedulable(node)

        if is_ready and is_schedulable:
            ready_nodes.append(node_name)
            print(f"✅ Node {node_name} is Ready and schedulable")
        else:
            status_msg = "Ready" if is_ready else "NotReady"
            schedulable_msg = "schedulable" if is_schedulable else "unschedulable"
            print(f"⚠️  Node {node_name} is {status_msg} and {schedulable_msg}")

    if len(ready_nodes) == 0:
        raise AssertionError(
            f"No Ready and schedulable nodes found in the cluster. "
            f"Total nodes: {len(nodes.items)}, Ready nodes: 0"
        )

    print(f"✅ Cluster has {len(ready_nodes)} Ready and schedulable node(s) out of {len(nodes.items)} total nodes")

# ----------------------------------------------------------------------------
# When steps (build desired deployment spec and apply on label step)
# ----------------------------------------------------------------------------

@when(parsers.parse("我创建 {replica_count:d} 个 {qos_type} 类型的 Deployment"))
@when(parsers.parse("I create {replica_count:d} {qos_type} Deployments"))
def create_qos_deployments(replica_count: int, qos_type: str):
    test_context["replica_count"] = replica_count
    test_context["qos_type"] = qos_type.lower()

@when(parsers.parse("Deployment 具有 {cpu_request}/{cpu_limit} CPU 资源配置"))
@when(parsers.parse("each Deployment requests/limits {cpu_request}/{cpu_limit} CPU"))
def deployment_has_cpu_config(cpu_request: str, cpu_limit: str):
    test_context["resource_config"]["cpu_request"] = cpu_request
    test_context["resource_config"]["cpu_limit"] = cpu_limit

@when(parsers.parse("Deployment 具有 {memory_request}/{memory_limit} Memory 资源配置"))
@when(parsers.parse("each Deployment requests/limits {memory_request}/{memory_limit} Memory"))
def deployment_has_memory_config(memory_request: str, memory_limit: str):
    test_context["resource_config"]["memory_request"] = memory_request
    test_context["resource_config"]["memory_limit"] = memory_limit

@when(parsers.parse('Deployment 具有标签 "{test_label}"'))
@when(parsers.parse('each Deployment has label "{test_label}"'))
def deployment_has_label(test_label: str, test_namespace: str):
    """Set test label and actually create Deployments in the cluster."""
    test_context["test_label"] = test_label
    _create_actual_deployments(test_namespace)

# ----------------------------------------------------------------------------
# Then steps (generic validations)
# ----------------------------------------------------------------------------

@then("容器应该被成功调度")
@then("all containers should be scheduled successfully")
def containers_should_be_scheduled(test_namespace: str):
    _wait_for_pods_ready(test_namespace)
    pods = _get_test_pods(test_namespace)
    for pod in pods:
        assert pod.status.phase == "Running", f"Pod {pod.metadata.name} is not Running: {pod.status.phase}"

@then(parsers.parse("容器的 QoS 类别应该是 {expected_qos}"))
@then(parsers.parse("the containers' QoS class should be {expected_qos}"))
def verify_qos_class(expected_qos: str, test_namespace: str):
    pods = _get_test_pods(test_namespace)
    for pod in pods:
        actual_qos = pod.status.qos_class
        assert actual_qos == expected_qos, (
            f"Pod {pod.metadata.name} QoS mismatch: expected {expected_qos}, got {actual_qos}"
        )

# ----------------------------------------------------------------------------
# Helpers (generic, no project-specific logic)
# ----------------------------------------------------------------------------

def _create_actual_deployments(namespace: str):
    qos_type: str = test_context.get("qos_type", "besteffort")
    replica_count: int = test_context.get("replica_count", 1)
    test_label: str = test_context.get("test_label", "bdd-test")
    resource_config: Dict[str, str] = test_context.get("resource_config", {})

    container_spec = _build_container_spec(qos_type, resource_config)
    apps = _apps_api()

    for i in range(replica_count):
        name = f"{test_label}-{i}"
        body = _build_deployment(name, test_label, container_spec)
        try:
            apps.create_namespaced_deployment(namespace=namespace, body=body)
        except ApiException as e:  # pragma: no cover
            raise AssertionError(f"Failed to create Deployment {name}: {e}")


def _build_container_spec(qos_type: str, resource_config: Dict[str, str]) -> Dict[str, Any]:
    container_spec: Dict[str, Any] = {
        "name": "test-container",
        "image": os.getenv("TEST_CONTAINER_IMAGE", "nginx:latest"),
        "image_pull_policy": "IfNotPresent",  # 默认优先使用本地镜像
        "resources": {},
    }

    if qos_type == "guaranteed":
        container_spec["resources"] = {
            "requests": {
                "cpu": resource_config.get("cpu_request", "1"),
                "memory": resource_config.get("memory_request", "2Gi"),
            },
            "limits": {
                "cpu": resource_config.get("cpu_limit", "1"),
                "memory": resource_config.get("memory_limit", "2Gi"),
            },
        }
    elif qos_type == "burstable":
        container_spec["resources"] = {
            "requests": {
                "cpu": resource_config.get("cpu_request", "0.5"),
                "memory": resource_config.get("memory_request", "1Gi"),
            },
            "limits": {
                "cpu": resource_config.get("cpu_limit", "1"),
                "memory": resource_config.get("memory_limit", "2Gi"),
            },
        }
    # besteffort: no resources

    return container_spec


def _build_deployment(name: str, test_label: str, container_spec: Dict[str, Any]) -> client.V1Deployment:
    return client.V1Deployment(
        metadata=client.V1ObjectMeta(name=name, labels={"test-label": test_label}),
        spec=client.V1DeploymentSpec(
            replicas=1,
            selector=client.V1LabelSelector(match_labels={"app": name}),
            template=client.V1PodTemplateSpec(
                metadata=client.V1ObjectMeta(labels={"app": name, "test-label": test_label}),
                spec=client.V1PodSpec(containers=[client.V1Container(**container_spec)]),
            ),
        ),
    )


def _wait_for_pods_ready(namespace: str, timeout: int = 300):
    start = time.time()
    while time.time() - start < timeout:
        pods = _get_test_pods(namespace)
        if pods and all(p.status.phase == "Running" for p in pods):
            return
        time.sleep(5)
    raise AssertionError(f"Timed out waiting for Pods to be Running ({timeout}s)")


def _get_test_pods(namespace: str) -> List[client.V1Pod]:
    test_label = test_context.get("test_label")
    if not test_label:
        return []
    pods = _core_api().list_namespaced_pod(namespace=namespace, label_selector=f"test-label={test_label}")
    return pods.items

# ----------------------------------------------------------------------------
# Cleanup utility (can be used from tests)
# ----------------------------------------------------------------------------

def cleanup_test_resources(test_namespace: str = None):
    """Delete created Deployments by label and reset context."""
    namespace = test_namespace or os.getenv("TEST_NAMESPACE", "default")
    label = test_context.get("test_label")
    if not label:
        return
    try:
        deployments = _apps_api().list_namespaced_deployment(namespace=namespace, label_selector=f"test-label={label}")
        for d in deployments.items:
            _apps_api().delete_namespaced_deployment(name=d.metadata.name, namespace=namespace)
    except Exception as e:  # pragma: no cover
        print(f"Cleanup warning: {e}")

    # reset context
    test_context.clear()
    test_context.update({
        "deployments": [],
        "pods": [],
        "test_label": None,
        "replica_count": 0,
        "resource_config": {},
    })

