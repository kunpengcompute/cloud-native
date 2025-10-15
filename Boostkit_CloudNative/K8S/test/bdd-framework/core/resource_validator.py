"""
通用资源验证器

提供资源分配和状态验证的基础功能
"""

import logging
from typing import Dict, Any, List, Optional
from kubernetes import client

from .base_manager import BaseManager

logger = logging.getLogger(__name__)

class ResourceValidator(BaseManager):
    """通用资源验证器"""
    
    def __init__(self, test_config: Dict[str, Any]):
        super().__init__(test_config)
        self.v1 = client.CoreV1Api()
        
    def get_required_config_keys(self) -> list:
        """获取必需的配置键"""
        return ["namespace"]
    
    def validate_pod_status(self, namespace: str, pod_name: str, expected_status: str) -> bool:
        """验证 Pod 状态"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            actual_status = pod.status.phase
            
            if actual_status == expected_status:
                self.log_info(f"Pod {pod_name} status validation passed: {actual_status}")
                return True
            else:
                self.log_error(f"Pod {pod_name} status validation failed: expected {expected_status}, got {actual_status}")
                return False
                
        except Exception as e:
            self.log_error(f"Failed to validate pod status for {pod_name}: {e}")
            return False
    
    def validate_pod_resources(self, namespace: str, pod_name: str, 
                              expected_resources: Dict[str, str]) -> bool:
        """验证 Pod 资源分配"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            
            if not pod.spec.containers:
                self.log_error(f"Pod {pod_name} has no containers")
                return False
                
            container = pod.spec.containers[0]
            if not container.resources or not container.resources.requests:
                self.log_error(f"Pod {pod_name} has no resource requests")
                return False
                
            actual_resources = dict(container.resources.requests)
            
            for resource, expected_value in expected_resources.items():
                if resource not in actual_resources:
                    self.log_error(f"Resource {resource} not found in pod {pod_name}")
                    return False
                    
                if actual_resources[resource] != expected_value:
                    self.log_error(f"Resource {resource} mismatch in pod {pod_name}: expected {expected_value}, got {actual_resources[resource]}")
                    return False
            
            self.log_info(f"Resource allocation validated for pod {pod_name}")
            return True
            
        except Exception as e:
            self.log_error(f"Failed to validate resource allocation for {pod_name}: {e}")
            return False
    
    def validate_pod_node_assignment(self, namespace: str, pod_name: str, 
                                   expected_node: str = None) -> bool:
        """验证 Pod 节点分配"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            actual_node = pod.spec.node_name
            
            if expected_node:
                if actual_node == expected_node:
                    self.log_info(f"Pod {pod_name} node assignment validated: {actual_node}")
                    return True
                else:
                    self.log_error(f"Pod {pod_name} node assignment failed: expected {expected_node}, got {actual_node}")
                    return False
            else:
                # 只验证是否分配了节点
                if actual_node:
                    self.log_info(f"Pod {pod_name} is assigned to node: {actual_node}")
                    return True
                else:
                    self.log_error(f"Pod {pod_name} is not assigned to any node")
                    return False
                    
        except Exception as e:
            self.log_error(f"Failed to validate node assignment for {pod_name}: {e}")
            return False
    
    def validate_pod_labels(self, namespace: str, pod_name: str, 
                           expected_labels: Dict[str, str]) -> bool:
        """验证 Pod 标签"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            actual_labels = pod.metadata.labels or {}
            
            for label_key, expected_value in expected_labels.items():
                if label_key not in actual_labels:
                    self.log_error(f"Label {label_key} not found in pod {pod_name}")
                    return False
                    
                if actual_labels[label_key] != expected_value:
                    self.log_error(f"Label {label_key} mismatch in pod {pod_name}: expected {expected_value}, got {actual_labels[label_key]}")
                    return False
            
            self.log_info(f"Labels validated for pod {pod_name}")
            return True
            
        except Exception as e:
            self.log_error(f"Failed to validate labels for {pod_name}: {e}")
            return False
    
    def validate_pod_annotations(self, namespace: str, pod_name: str, 
                                expected_annotations: Dict[str, str]) -> bool:
        """验证 Pod 注解"""
        try:
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            actual_annotations = pod.metadata.annotations or {}
            
            for annotation_key, expected_value in expected_annotations.items():
                if annotation_key not in actual_annotations:
                    self.log_error(f"Annotation {annotation_key} not found in pod {pod_name}")
                    return False
                    
                if actual_annotations[annotation_key] != expected_value:
                    self.log_error(f"Annotation {annotation_key} mismatch in pod {pod_name}: expected {expected_value}, got {actual_annotations[annotation_key]}")
                    return False
            
            self.log_info(f"Annotations validated for pod {pod_name}")
            return True
            
        except Exception as e:
            self.log_error(f"Failed to validate annotations for {pod_name}: {e}")
            return False
    
    def get_pod_resource_usage(self, namespace: str, pod_name: str) -> Dict[str, Any]:
        """获取 Pod 资源使用情况"""
        try:
            # 这里需要 metrics-server 支持
            # 暂时返回空字典，具体实现可以根据需要扩展
            self.log_warning("Resource usage metrics not implemented yet")
            return {}
            
        except Exception as e:
            self.log_error(f"Failed to get resource usage for {pod_name}: {e}")
            return {}
    
    def validate_namespace_resource_quota(self, namespace: str) -> bool:
        """验证命名空间资源配额"""
        try:
            # 获取资源配额
            quotas = self.v1.list_namespaced_resource_quota(namespace)
            
            if not quotas.items:
                self.log_info(f"No resource quotas found in namespace {namespace}")
                return True
            
            for quota in quotas.items:
                quota_name = quota.metadata.name
                status = quota.status
                
                if status and status.hard and status.used:
                    for resource, hard_limit in status.hard.items():
                        used = status.used.get(resource, "0")
                        self.log_info(f"Quota {quota_name} - {resource}: {used}/{hard_limit}")
            
            return True
            
        except Exception as e:
            self.log_error(f"Failed to validate resource quota for namespace {namespace}: {e}")
            return False
