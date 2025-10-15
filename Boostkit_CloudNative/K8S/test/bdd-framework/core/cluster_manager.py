"""
通用集群管理器

提供 Kubernetes 集群的基础操作功能
"""

import logging
from typing import Dict, Any, List, Optional
from kubernetes import client, config
from kubernetes.client.rest import ApiException

from .base_manager import BaseManager

logger = logging.getLogger(__name__)

class ClusterManager(BaseManager):
    """通用 Kubernetes 集群管理器"""
    
    def __init__(self, test_config: Dict[str, Any]):
        super().__init__(test_config)
        self._init_kubernetes_client()
        
    def get_required_config_keys(self) -> list:
        """获取必需的配置键"""
        return ["cluster_config"]
    
    def _init_kubernetes_client(self) -> None:
        """初始化 Kubernetes 客户端"""
        try:
            kubeconfig_path = self.get_config("cluster_config")
            if kubeconfig_path:
                config.load_kube_config(kubeconfig_path)
            else:
                config.load_incluster_config()
                
            self.v1 = client.CoreV1Api()
            self.apps_v1 = client.AppsV1Api()
            self.log_info("Kubernetes client initialized successfully")
            
        except Exception as e:
            self.log_error(f"Failed to initialize Kubernetes client: {e}")
            raise
    
    def create_namespace(self, namespace: str) -> None:
        """创建命名空间"""
        try:
            namespace_obj = client.V1Namespace(
                metadata=client.V1ObjectMeta(name=namespace)
            )
            self.v1.create_namespace(namespace_obj)
            self.log_info(f"Created namespace: {namespace}")
        except ApiException as e:
            if e.status == 409:  # Already exists
                self.log_info(f"Namespace {namespace} already exists")
            else:
                raise
    
    def delete_namespace(self, namespace: str) -> None:
        """删除命名空间"""
        try:
            self.v1.delete_namespace(namespace)
            self.log_info(f"Deleted namespace: {namespace}")
        except ApiException as e:
            if e.status == 404:  # Not found
                self.log_info(f"Namespace {namespace} not found")
            else:
                raise
    
    def namespace_exists(self, namespace: str) -> bool:
        """检查命名空间是否存在"""
        try:
            self.v1.read_namespace(namespace)
            return True
        except ApiException as e:
            if e.status == 404:
                return False
            else:
                raise
    
    def get_nodes(self) -> List[client.V1Node]:
        """获取集群节点列表"""
        try:
            nodes = self.v1.list_node()
            return nodes.items
        except Exception as e:
            self.log_error(f"Failed to get nodes: {e}")
            raise
    
    def get_node_info(self, node_name: str) -> Optional[client.V1Node]:
        """获取特定节点信息"""
        try:
            node = self.v1.read_node(node_name)
            return node
        except ApiException as e:
            if e.status == 404:
                self.log_warning(f"Node {node_name} not found")
                return None
            else:
                raise
    
    def get_container_runtime_info(self) -> str:
        """获取容器运行时信息"""
        try:
            nodes = self.get_nodes()
            if nodes:
                node = nodes[0]
                runtime_version = node.status.node_info.container_runtime_version
                self.log_info(f"Container runtime: {runtime_version}")
                return runtime_version
            else:
                raise RuntimeError("No nodes found in cluster")
        except Exception as e:
            self.log_error(f"Failed to get container runtime info: {e}")
            raise
    
    def get_pods_in_namespace(self, namespace: str) -> List[client.V1Pod]:
        """获取命名空间中的所有 Pod"""
        try:
            pods = self.v1.list_namespaced_pod(namespace)
            return pods.items
        except Exception as e:
            self.log_error(f"Failed to get pods in namespace {namespace}: {e}")
            raise
    
    def wait_for_pod_ready(self, namespace: str, pod_name: str, timeout: int = 300) -> bool:
        """等待 Pod 就绪"""
        import time
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            try:
                pod = self.v1.read_namespaced_pod(pod_name, namespace)
                if pod.status.phase == "Running":
                    # 检查所有容器是否就绪
                    if pod.status.container_statuses:
                        all_ready = all(status.ready for status in pod.status.container_statuses)
                        if all_ready:
                            self.log_info(f"Pod {pod_name} is ready")
                            return True
                
                time.sleep(5)
            except ApiException as e:
                if e.status == 404:
                    self.log_warning(f"Pod {pod_name} not found")
                    time.sleep(5)
                else:
                    raise
        
        self.log_error(f"Pod {pod_name} did not become ready within {timeout} seconds")
        return False
    
    def cleanup_namespace_resources(self, namespace: str) -> None:
        """清理命名空间中的资源"""
        try:
            # 删除所有 Pod
            self.v1.delete_collection_namespaced_pod(namespace)
            
            # 删除所有 Deployment
            self.apps_v1.delete_collection_namespaced_deployment(namespace)
            
            # 删除所有 DaemonSet
            self.apps_v1.delete_collection_namespaced_daemon_set(namespace)
            
            self.log_info(f"Cleaned up resources in namespace: {namespace}")
        except Exception as e:
            self.log_error(f"Failed to cleanup namespace resources: {e}")
            raise
    
    def check_cluster_connectivity(self) -> bool:
        """检查集群连接性"""
        try:
            # 尝试获取集群版本信息
            version = self.v1.get_api_resources()
            self.log_info("Cluster connectivity check passed")
            return True
        except Exception as e:
            self.log_error(f"Cluster connectivity check failed: {e}")
            return False
