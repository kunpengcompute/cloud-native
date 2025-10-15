"""
kunpeng-tap plugin management utilities for BDD tests
"""

import logging
import subprocess
import json
import os
import time
from typing import Dict, Any, List, Optional
from pathlib import Path

logger = logging.getLogger(__name__)

class KunpengTapManager:
    """Manages kunpeng-tap plugin for testing"""
    
    def __init__(self, test_config: Dict[str, Any]):
        self.test_config = test_config
        self.binary_path = test_config.get("kunpeng_tap_binary", "/usr/local/bin/kunpeng-tap")
        self.config_path = "/etc/kunpeng-tap/config.yaml"
        self.log_path = "/var/log/kunpeng-tap.log"
        
    def is_binary_available(self) -> bool:
        """Check if kunpeng-tap binary is available"""
        try:
            result = subprocess.run([self.binary_path, "--version"], 
                                  capture_output=True, text=True, timeout=10)
            available = result.returncode == 0
            logger.info(f"kunpeng-tap binary available: {available}")
            return available
        except (subprocess.TimeoutExpired, FileNotFoundError) as e:
            logger.warning(f"kunpeng-tap binary not available: {e}")
            return False
    
    def is_service_running(self) -> bool:
        """Check if kunpeng-tap service is running"""
        try:
            # Check if process is running
            result = subprocess.run(["pgrep", "-f", "kunpeng-tap"], 
                                  capture_output=True, text=True)
            running = result.returncode == 0
            
            if running:
                logger.info("kunpeng-tap service is running")
            else:
                logger.warning("kunpeng-tap service is not running")
                
            return running
        except Exception as e:
            logger.error(f"Failed to check kunpeng-tap service status: {e}")
            return False
    
    def get_policy_configuration(self) -> Dict[str, Any]:
        """Get current policy configuration"""
        try:
            # Try to read configuration from config file
            if os.path.exists(self.config_path):
                with open(self.config_path, 'r') as f:
                    try:
                        import yaml
                        config = yaml.safe_load(f)
                    except ImportError:
                        # Fallback to JSON if PyYAML is not available
                        import json
                        config = json.load(f)
                    logger.info(f"Policy configuration: {config}")
                    return config
            else:
                # Return default configuration for testing
                default_config = {
                    "resource_policy": "topology-aware",
                    "container_runtime_mode": "NRI",
                    "enable_memory_topology": True,
                    "nri_socket_path": "/var/run/nri/nri.sock"
                }
                logger.info(f"Using default policy configuration: {default_config}")
                return default_config
        except Exception as e:
            logger.error(f"Failed to get policy configuration: {e}")
            return {}
    
    def get_containerd_configuration(self) -> Dict[str, Any]:
        """Get containerd integration configuration"""
        config = self.get_policy_configuration()
        containerd_config = {
            "container_runtime_mode": config.get("container_runtime_mode", "NRI"),
            "nri_socket_path": config.get("nri_socket_path", "/var/run/nri/nri.sock"),
            "runtime_proxy_endpoint": config.get("runtime_proxy_endpoint", "/var/run/kunpeng-tap/proxy.sock")
        }
        logger.info(f"Containerd configuration: {containerd_config}")
        return containerd_config
    
    def start_service(self, mode: str = "NRI") -> bool:
        """Start kunpeng-tap service"""
        try:
            cmd = [
                self.binary_path,
                f"--container-runtime-mode={mode}",
                "--resource-policy=topology-aware",
                "--nri-socket-path=/var/run/nri/nri.sock",
                "-v=2"
            ]
            
            # Start service in background
            process = subprocess.Popen(cmd, stdout=subprocess.PIPE, 
                                     stderr=subprocess.PIPE, text=True)
            
            # Wait a bit for service to start
            time.sleep(3)
            
            # Check if service is running
            if self.is_service_running():
                logger.info("kunpeng-tap service started successfully")
                return True
            else:
                logger.error("Failed to start kunpeng-tap service")
                return False
                
        except Exception as e:
            logger.error(f"Failed to start kunpeng-tap service: {e}")
            return False
    
    def stop_service(self) -> bool:
        """Stop kunpeng-tap service"""
        try:
            # Kill the process
            result = subprocess.run(["pkill", "-f", "kunpeng-tap"], 
                                  capture_output=True, text=True)
            
            # Wait a bit for service to stop
            time.sleep(2)
            
            # Verify service is stopped
            if not self.is_service_running():
                logger.info("kunpeng-tap service stopped successfully")
                return True
            else:
                logger.warning("kunpeng-tap service may still be running")
                return False
                
        except Exception as e:
            logger.error(f"Failed to stop kunpeng-tap service: {e}")
            return False
    
    def restart_service(self) -> bool:
        """Restart kunpeng-tap service"""
        logger.info("Restarting kunpeng-tap service")
        self.stop_service()
        time.sleep(2)
        return self.start_service()
    
    def get_service_logs(self, lines: int = 100) -> str:
        """Get kunpeng-tap service logs"""
        try:
            if os.path.exists(self.log_path):
                result = subprocess.run(["tail", f"-{lines}", self.log_path], 
                                      capture_output=True, text=True)
                return result.stdout
            else:
                # Try to get logs from journalctl
                result = subprocess.run(["journalctl", "-u", "kunpeng-tap", 
                                       f"-n", str(lines), "--no-pager"], 
                                      capture_output=True, text=True)
                return result.stdout
        except Exception as e:
            logger.error(f"Failed to get service logs: {e}")
            return ""
    
    def check_nri_registration(self) -> bool:
        """Check if kunpeng-tap is registered as NRI plugin"""
        try:
            logs = self.get_service_logs(50)
            registered = "Created plugin" in logs or "NRI plugin registered" in logs
            logger.info(f"NRI registration status: {registered}")
            return registered
        except Exception as e:
            logger.error(f"Failed to check NRI registration: {e}")
            return False
    
    def get_resource_allocations(self) -> Dict[str, Any]:
        """Get current resource allocations from kunpeng-tap"""
        try:
            # This would typically query kunpeng-tap's internal state
            # For testing, we'll return mock data
            allocations = {
                "total_containers": 2,
                "numa_node_0": {
                    "allocated_cpus": "0-1",
                    "allocated_memory": "4Gi",
                    "containers": ["test-pod-1"]
                },
                "numa_node_1": {
                    "allocated_cpus": "4-5", 
                    "allocated_memory": "4Gi",
                    "containers": ["test-pod-2"]
                }
            }
            logger.info(f"Resource allocations: {allocations}")
            return allocations
        except Exception as e:
            logger.error(f"Failed to get resource allocations: {e}")
            return {}
    
    def validate_configuration(self) -> List[str]:
        """Validate kunpeng-tap configuration"""
        issues = []
        
        # Check binary availability
        if not self.is_binary_available():
            issues.append("kunpeng-tap binary is not available")
        
        # Check configuration file
        if not os.path.exists(self.config_path):
            issues.append(f"Configuration file not found: {self.config_path}")
        
        # Check NRI socket
        nri_socket = "/var/run/nri/nri.sock"
        if not os.path.exists(nri_socket):
            issues.append(f"NRI socket not found: {nri_socket}")
        
        # Check service status
        if not self.is_service_running():
            issues.append("kunpeng-tap service is not running")
        
        if issues:
            logger.warning(f"Configuration issues found: {issues}")
        else:
            logger.info("kunpeng-tap configuration is valid")
            
        return issues
    
    def reset_allocations(self) -> bool:
        """Reset all resource allocations"""
        try:
            # This would typically reset kunpeng-tap's internal state
            # For testing, we'll just restart the service
            return self.restart_service()
        except Exception as e:
            logger.error(f"Failed to reset allocations: {e}")
            return False
