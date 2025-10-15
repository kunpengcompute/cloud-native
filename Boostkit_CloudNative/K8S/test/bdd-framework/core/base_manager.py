"""
BDD 测试框架基础管理器

提供所有管理器类的基础功能和通用接口
"""

import logging
from abc import ABC, abstractmethod
from typing import Dict, Any, Optional

logger = logging.getLogger(__name__)

class BaseManager(ABC):
    """所有管理器类的基类"""
    
    def __init__(self, test_config: Dict[str, Any]):
        """
        初始化基础管理器
        
        Args:
            test_config: 测试配置字典
        """
        self.test_config = test_config
        self.logger = logging.getLogger(self.__class__.__name__)
        self._validate_config()
    
    def _validate_config(self) -> None:
        """验证测试配置"""
        required_keys = self.get_required_config_keys()
        missing_keys = [key for key in required_keys if key not in self.test_config]
        
        if missing_keys:
            raise ValueError(f"Missing required config keys: {missing_keys}")
    
    @abstractmethod
    def get_required_config_keys(self) -> list:
        """
        获取必需的配置键列表
        
        Returns:
            必需配置键的列表
        """
        pass
    
    def get_config(self, key: str, default: Any = None) -> Any:
        """
        获取配置值
        
        Args:
            key: 配置键
            default: 默认值
            
        Returns:
            配置值
        """
        return self.test_config.get(key, default)
    
    def set_config(self, key: str, value: Any) -> None:
        """
        设置配置值
        
        Args:
            key: 配置键
            value: 配置值
        """
        self.test_config[key] = value
    
    def log_info(self, message: str, **kwargs) -> None:
        """记录信息日志"""
        self.logger.info(message, **kwargs)
    
    def log_warning(self, message: str, **kwargs) -> None:
        """记录警告日志"""
        self.logger.warning(message, **kwargs)
    
    def log_error(self, message: str, **kwargs) -> None:
        """记录错误日志"""
        self.logger.error(message, **kwargs)
    
    def log_debug(self, message: str, **kwargs) -> None:
        """记录调试日志"""
        self.logger.debug(message, **kwargs)

class ConfigurableManager(BaseManager):
    """可配置的管理器基类"""
    
    def __init__(self, test_config: Dict[str, Any], manager_config: Optional[Dict[str, Any]] = None):
        """
        初始化可配置管理器
        
        Args:
            test_config: 测试配置
            manager_config: 管理器特定配置
        """
        super().__init__(test_config)
        self.manager_config = manager_config or {}
    
    def get_manager_config(self, key: str, default: Any = None) -> Any:
        """
        获取管理器特定配置值
        
        Args:
            key: 配置键
            default: 默认值
            
        Returns:
            配置值
        """
        return self.manager_config.get(key, default)
    
    def set_manager_config(self, key: str, value: Any) -> None:
        """
        设置管理器特定配置值
        
        Args:
            key: 配置键
            value: 配置值
        """
        self.manager_config[key] = value
