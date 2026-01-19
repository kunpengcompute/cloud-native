"""
通用容器管理器

提供容器部署和生命周期管理的基础功能
"""

import logging
import time
from typing import Dict, Any, List, Optional
from kubernetes import client
from kubernetes.client.rest import ApiException

from .base_manager import BaseManager

logger = logging.getLogger(__name__)

class ContainerManager(BaseManager):
    """通用容器管理器"""
    
    def __init__(self, test_config: Dict[str, Any]):
        super().__init__(test_config)
        self.v1 = client.CoreV1Api()
        self.apps_v1 = client.AppsV1Api()
        
    def get_required_config_keys(self) -> list:
        """获取必需的配置键"""
        return ["namespace"]
    
    def parse_resource_table(self, resource_table: str) -> Dict[str, str]:
        """解析资源表格"""
        resources = {}
        lines = resource_table.strip().split('\n')
        
        for line in lines[1:]:  # 跳过表头
            if '|' in line:
                parts = [part.strip() for part in line.split('|') if part.strip()]
                if len(parts) >= 2:
                    resource_type = parts[0]
                    value = parts[1]
                    resources[resource_type] = value
                    
        self.log_info(f"Parsed resources: {resources}")
        return resources
    
    def create_pod_spec(self, namespace: str, container_spec: Dict[str, Any]) -> client.V1Pod:
        """创建 Pod 规范"""
        
        # 解析资源
        resources = container_spec.get('resources', {})
        resource_requirements = client.V1ResourceRequirements(
            requests=resources,
            limits=resources
        )
        
        # 创建容器
        container = client.V1Container(
            name=container_spec['name'],
            image=container_spec['image'],
            image_pull_policy=container_spec.get('image_pull_policy', 'IfNotPresent'),  # 默认使用本地镜像
            command=container_spec.get('command'),
            resources=resource_requirements,
            env=container_spec.get('env', [])
        )
        
        # 创建 Pod 规范
        pod_spec = client.V1PodSpec(
            containers=[container],
            restart_policy=container_spec.get('restart_policy', "Never"),
            node_selector=container_spec.get('node_selector'),
            affinity=container_spec.get('affinity')
        )
        
        # 创建元数据
        annotations = container_spec.get('annotations', {})
        labels = container_spec.get('labels', {})
        
        metadata = client.V1ObjectMeta(
            name=container_spec['name'],
            namespace=namespace,
            annotations=annotations,
            labels=labels
        )
        
        # 创建 Pod
        pod = client.V1Pod(
            api_version="v1",
            kind="Pod",
            metadata=metadata,
            spec=pod_spec
        )
        
        return pod
    
    def deploy_pod(self, namespace: str, container_spec: Dict[str, Any]) -> str:
        """部署 Pod"""
        try:
            pod = self.create_pod_spec(namespace, container_spec)
            
            # 创建 Pod
            created_pod = self.v1.create_namespaced_pod(namespace, pod)
            pod_name = created_pod.metadata.name
            
            self.log_info(f"Deployed pod: {pod_name}")
            return pod_name
            
        except Exception as e:
            self.log_error(f"Failed to deploy pod: {e}")
            raise
    
    def delete_pod(self, namespace: str, pod_name: str) -> None:
        """删除 Pod"""
        try:
            self.v1.delete_namespaced_pod(pod_name, namespace)
            self.log_info(f"Deleted pod: {pod_name}")
            
        except ApiException as e:
            if e.status == 404:
                self.log_info(f"Pod {pod_name} not found")
            else:
                raise
    
    def get_pod_status(self, namespace: str, pod_name: str) -> str:
        """获取 Pod 状态"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            return pod.status.phase
            
        except Exception as e:
            self.log_error(f"Failed to get pod status for {pod_name}: {e}")
            return "Unknown"
    
    def get_pod_node(self, namespace: str, pod_name: str) -> str:
        """获取 Pod 所在节点"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            return pod.spec.node_name or ""
            
        except Exception as e:
            self.log_error(f"Failed to get pod node for {pod_name}: {e}")
            return ""
    
    def get_pod_info(self, namespace: str, pod_name: str) -> Optional[client.V1Pod]:
        """获取 Pod 详细信息"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            return pod
        except ApiException as e:
            if e.status == 404:
                self.log_warning(f"Pod {pod_name} not found")
                return None
            else:
                raise
    
    def wait_for_pod_scheduled(self, namespace: str, pod_name: str, timeout: int = 60) -> bool:
        """等待 Pod 被调度"""
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            try:
                pod = self.v1.read_namespaced_pod(pod_name, namespace)
                if pod.spec.node_name:
                    self.log_info(f"Pod {pod_name} scheduled to node {pod.spec.node_name}")
                    return True
                time.sleep(2)
            except ApiException:
                time.sleep(2)
        
        self.log_error(f"Pod {pod_name} was not scheduled within {timeout} seconds")
        return False
    
    def wait_for_pod_running(self, namespace: str, pod_name: str, timeout: int = 300) -> bool:
        """等待 Pod 运行"""
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            try:
                pod = self.v1.read_namespaced_pod(pod_name, namespace)
                if pod.status.phase == "Running":
                    # 检查所有容器是否就绪
                    if pod.status.container_statuses:
                        all_ready = all(status.ready for status in pod.status.container_statuses)
                        if all_ready:
                            self.log_info(f"Pod {pod_name} is running and ready")
                            return True
                
                time.sleep(5)
            except ApiException as e:
                if e.status == 404:
                    self.log_warning(f"Pod {pod_name} not found")
                    time.sleep(5)
                else:
                    raise
        
        self.log_error(f"Pod {pod_name} did not become running within {timeout} seconds")
        return False
    
    def get_pod_logs(self, namespace: str, pod_name: str, container_name: str = None) -> str:
        """获取 Pod 日志"""
        try:
            logs = self.v1.read_namespaced_pod_log(
                name=pod_name,
                namespace=namespace,
                container=container_name
            )
            return logs
        except Exception as e:
            self.log_error(f"Failed to get logs for pod {pod_name}: {e}")
            return ""
    
    def execute_in_pod(self, namespace: str, pod_name: str, command: List[str], 
                      container_name: str = None) -> str:
        """在 Pod 中执行命令"""
        try:
            from kubernetes.stream import stream
            
            resp = stream(
                self.v1.connect_get_namespaced_pod_exec,
                pod_name,
                namespace,
                command=command,
                container=container_name,
                stderr=True,
                stdin=False,
                stdout=True,
                tty=False
            )
            return resp
        except Exception as e:
            self.log_error(f"Failed to execute command in pod {pod_name}: {e}")
            raise
