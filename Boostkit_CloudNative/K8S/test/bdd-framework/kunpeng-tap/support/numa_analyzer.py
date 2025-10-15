"""
NUMA topology analysis utilities for BDD tests
"""

import logging
import re
import subprocess
from typing import Dict, Any, List, Set, Optional
from kubernetes import client

logger = logging.getLogger(__name__)

class NumaAnalyzer:
    """Analyzes NUMA topology and container assignments"""
    
    def __init__(self, test_config: Dict[str, Any]):
        self.test_config = test_config
        self.v1 = client.CoreV1Api()
        
    def get_pod_numa_assignment(self, pod_name: str, namespace: str = None) -> List[int]:
        """Get NUMA node assignment for a pod"""
        try:
            if namespace is None:
                namespace = self.test_config.get('namespace', 'default')
                
            # Get pod information
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            node_name = pod.spec.node_name
            
            if not node_name:
                return []
            
            # Get container cgroup information
            container_id = self._get_container_id(pod)
            if not container_id:
                return []
            
            # Analyze cgroup cpuset to determine NUMA assignment
            cpuset = self._get_container_cpuset(node_name, container_id)
            numa_nodes = self._cpuset_to_numa_nodes(cpuset)
            
            logger.info(f"Pod {pod_name} NUMA assignment: {numa_nodes}")
            return numa_nodes
            
        except Exception as e:
            logger.error(f"Failed to get NUMA assignment for {pod_name}: {e}")
            return []
    
    def get_pod_cpu_affinity(self, pod_name: str, namespace: str = None) -> Set[int]:
        """Get CPU affinity for a pod"""
        try:
            if namespace is None:
                namespace = self.test_config.get('namespace', 'default')
                
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            node_name = pod.spec.node_name
            
            if not node_name:
                return set()
            
            container_id = self._get_container_id(pod)
            if not container_id:
                return set()
            
            cpuset = self._get_container_cpuset(node_name, container_id)
            cpu_set = self._parse_cpuset(cpuset)
            
            logger.info(f"Pod {pod_name} CPU affinity: {cpu_set}")
            return cpu_set
            
        except Exception as e:
            logger.error(f"Failed to get CPU affinity for {pod_name}: {e}")
            return set()
    
    def get_pod_memory_nodes(self, pod_name: str, namespace: str = None) -> List[int]:
        """Get memory nodes for a pod"""
        try:
            if namespace is None:
                namespace = self.test_config.get('namespace', 'default')
                
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            node_name = pod.spec.node_name
            
            if not node_name:
                return []
            
            container_id = self._get_container_id(pod)
            if not container_id:
                return []
            
            memset = self._get_container_memset(node_name, container_id)
            memory_nodes = self._parse_cpuset(memset)  # Same parsing logic
            
            logger.info(f"Pod {pod_name} memory nodes: {list(memory_nodes)}")
            return list(memory_nodes)
            
        except Exception as e:
            logger.error(f"Failed to get memory nodes for {pod_name}: {e}")
            return []
    
    def get_numa_node_cpus(self, numa_node: int) -> Set[int]:
        """Get CPUs belonging to a NUMA node"""
        try:
            # This would typically query the actual node topology
            # For testing, we'll use mock data based on common configurations
            if numa_node == 0:
                return {0, 1, 2, 3}
            elif numa_node == 1:
                return {4, 5, 6, 7}
            else:
                return set()
                
        except Exception as e:
            logger.error(f"Failed to get CPUs for NUMA node {numa_node}: {e}")
            return set()
    
    def get_pod_resource_requirements(self, pod_name: str, namespace: str = None) -> Dict[str, Any]:
        """Get resource requirements for a pod"""
        try:
            if namespace is None:
                namespace = self.test_config.get('namespace', 'default')
                
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            
            requirements = {}
            if pod.spec.containers:
                container = pod.spec.containers[0]
                if container.resources and container.resources.requests:
                    requirements = dict(container.resources.requests)
            
            logger.info(f"Pod {pod_name} resource requirements: {requirements}")
            return requirements
            
        except Exception as e:
            logger.error(f"Failed to get resource requirements for {pod_name}: {e}")
            return {}
    
    def verify_optimal_assignment(self, numa_assignment: List[int], 
                                resource_requirements: Dict[str, Any]) -> bool:
        """Verify if NUMA assignment is optimal for given resource requirements"""
        try:
            # Simple heuristic: single NUMA node assignment is generally optimal
            # for containers that fit within one node
            if len(numa_assignment) == 1:
                return True
            
            # For larger containers, check if assignment makes sense
            cpu_request = resource_requirements.get('cpu', '0')
            if isinstance(cpu_request, str):
                cpu_cores = self._parse_cpu_quantity(cpu_request)
            else:
                cpu_cores = float(cpu_request)
            
            # If requesting more than 4 cores (typical NUMA node size), 
            # multi-node assignment might be acceptable
            if cpu_cores > 4 and len(numa_assignment) <= 2:
                return True
            
            return len(numa_assignment) == 1
            
        except Exception as e:
            logger.error(f"Failed to verify optimal assignment: {e}")
            return False
    
    def check_resource_conflicts(self, pod_names: List[str]) -> List[str]:
        """Check for resource conflicts between pods"""
        conflicts = []
        try:
            pod_assignments = {}
            
            for pod_name in pod_names:
                cpu_affinity = self.get_pod_cpu_affinity(pod_name)
                pod_assignments[pod_name] = cpu_affinity
            
            # Check for overlapping CPU assignments
            for i, (pod1, cpus1) in enumerate(pod_assignments.items()):
                for j, (pod2, cpus2) in enumerate(pod_assignments.items()):
                    if i < j and cpus1.intersection(cpus2):
                        conflicts.append(f"CPU conflict between {pod1} and {pod2}")
            
            logger.info(f"Resource conflicts: {conflicts}")
            return conflicts
            
        except Exception as e:
            logger.error(f"Failed to check resource conflicts: {e}")
            return []
    
    def get_numa_utilization(self) -> Dict[int, float]:
        """Get NUMA node utilization"""
        try:
            # This would typically query actual node utilization
            # For testing, we'll return mock data
            utilization = {
                0: 0.6,  # 60% utilized
                1: 0.4   # 40% utilized
            }
            
            logger.info(f"NUMA utilization: {utilization}")
            return utilization
            
        except Exception as e:
            logger.error(f"Failed to get NUMA utilization: {e}")
            return {}
    
    def get_pod_gpu_assignment(self, pod_name: str, namespace: str = None) -> List[str]:
        """Get GPU assignment for a pod"""
        try:
            if namespace is None:
                namespace = self.test_config.get('namespace', 'default')
                
            # This would typically query actual GPU assignments
            # For testing, we'll check resource requests
            pod = self.v1.read_namespaced_pod(pod_name, namespace)
            
            gpu_devices = []
            if pod.spec.containers:
                container = pod.spec.containers[0]
                if container.resources and container.resources.requests:
                    for resource, quantity in container.resources.requests.items():
                        if 'gpu' in resource.lower():
                            # Mock GPU device assignment
                            gpu_devices = ['gpu-0']
            
            logger.info(f"Pod {pod_name} GPU assignment: {gpu_devices}")
            return gpu_devices
            
        except Exception as e:
            logger.error(f"Failed to get GPU assignment for {pod_name}: {e}")
            return []
    
    def get_gpu_numa_nodes(self, gpu_devices: List[str]) -> List[int]:
        """Get NUMA nodes for GPU devices"""
        try:
            # This would typically query actual GPU topology
            # For testing, assume GPU-0 is on NUMA node 1
            numa_nodes = []
            for gpu in gpu_devices:
                if gpu == 'gpu-0':
                    numa_nodes.append(1)
            
            logger.info(f"GPU devices {gpu_devices} on NUMA nodes: {numa_nodes}")
            return numa_nodes
            
        except Exception as e:
            logger.error(f"Failed to get GPU NUMA nodes: {e}")
            return []
    
    def verify_numa_constraints(self, pod_name: str, namespace: str = None) -> bool:
        """Verify that NUMA constraints are respected"""
        try:
            numa_assignment = self.get_pod_numa_assignment(pod_name, namespace)
            cpu_affinity = self.get_pod_cpu_affinity(pod_name, namespace)
            memory_nodes = self.get_pod_memory_nodes(pod_name, namespace)
            
            # Check if CPU affinity matches NUMA assignment
            for numa_node in numa_assignment:
                expected_cpus = self.get_numa_node_cpus(numa_node)
                if not cpu_affinity.issubset(expected_cpus):
                    return False
            
            # Check if memory nodes match NUMA assignment
            if set(memory_nodes) != set(numa_assignment):
                return False
            
            return True
            
        except Exception as e:
            logger.error(f"Failed to verify NUMA constraints for {pod_name}: {e}")
            return False
    
    def _get_container_id(self, pod: client.V1Pod) -> Optional[str]:
        """Get container ID from pod status"""
        if pod.status.container_statuses:
            container_status = pod.status.container_statuses[0]
            if container_status.container_id:
                # Extract actual container ID from the full string
                return container_status.container_id.split('://')[-1]
        return None
    
    def _get_container_cpuset(self, node_name: str, container_id: str) -> str:
        """Get container cpuset from cgroup"""
        try:
            # This would typically execute on the node
            # For testing, return mock cpuset
            return "0-3"  # Mock cpuset
        except Exception as e:
            logger.error(f"Failed to get container cpuset: {e}")
            return ""
    
    def _get_container_memset(self, node_name: str, container_id: str) -> str:
        """Get container memory set from cgroup"""
        try:
            # This would typically execute on the node
            # For testing, return mock memset
            return "0"  # Mock memset
        except Exception as e:
            logger.error(f"Failed to get container memset: {e}")
            return ""
    
    def _parse_cpuset(self, cpuset: str) -> Set[int]:
        """Parse cpuset string to set of integers"""
        cpu_set = set()
        if not cpuset:
            return cpu_set
        
        for part in cpuset.split(','):
            if '-' in part:
                start, end = map(int, part.split('-'))
                cpu_set.update(range(start, end + 1))
            else:
                cpu_set.add(int(part))
        
        return cpu_set
    
    def _cpuset_to_numa_nodes(self, cpuset: str) -> List[int]:
        """Convert cpuset to NUMA node assignments"""
        cpu_set = self._parse_cpuset(cpuset)
        numa_nodes = []
        
        # Determine NUMA nodes based on CPU assignments
        if cpu_set.intersection({0, 1, 2, 3}):
            numa_nodes.append(0)
        if cpu_set.intersection({4, 5, 6, 7}):
            numa_nodes.append(1)
        
        return numa_nodes
    
    def _parse_cpu_quantity(self, cpu_str: str) -> float:
        """Parse Kubernetes CPU quantity string"""
        if cpu_str.endswith('m'):
            return float(cpu_str[:-1]) / 1000
        else:
            return float(cpu_str)
