"""
load the config from config.yaml and .env
"""

import os
import yaml
from pathlib import Path
from typing import Dict, Any


class Config:
    """Configuration loader that reads from config.yaml and environment variables."""
    
    def __init__(self, config_path: str = None):
        """Initialize configuration loader.
        
        Args:
            config_path: Path to config.yaml file. If None, looks for config.yaml 
                        in the same directory as this module.
        """
        if config_path is None:
            config_path = Path(__file__).parent / "config.yaml"
        
        self.config_path = Path(config_path)
        self._config = self._load_config()
    
    def _load_config(self) -> Dict[str, Any]:
        """Load configuration from YAML file and override with environment variables."""
        # Load base configuration from YAML
        try:
            with open(self.config_path, 'r') as f:
                config = yaml.safe_load(f) or {}
        except FileNotFoundError:
            raise FileNotFoundError(f"Configuration file not found: {self.config_path}")
        except yaml.YAMLError as e:
            raise ValueError(f"Invalid YAML in configuration file: {e}")
        
        # Override with environment variables
        config = self._apply_env_overrides(config)
        
        return config
    
    def _apply_env_overrides(self, config: Dict[str, Any]) -> Dict[str, Any]:
        """Apply environment variable overrides to configuration."""
        # Environment variable mapping
        env_mappings = {
            'KAFKA_BOOTSTRAP_SERVERS': ('kafka', 'bootstrap_servers'),
            'KAFKA_GROUP_ID': ('kafka', 'consumer', 'group_id'),
            'MONGODB_URI': ('mongodb', 'uri'),
            'MONGODB_DATABASE': ('mongodb', 'database'),
            'FETCHER_USER_AGENT': ('fetcher', 'user_agent'),
            'FETCHER_TIMEOUT': ('fetcher', 'timeout'),
            'POLITENESS_DEFAULT_DELAY': ('politeness', 'default_delay'),
            'POLITENESS_RESPECT_ROBOTS': ('politeness', 'respect_robots_txt'),
            'LOG_LEVEL': ('logging', 'level'),
            'METRICS_ENABLED': ('metrics', 'enabled'),
            'METRICS_PORT': ('metrics', 'port'),
        }
        
        for env_var, config_path in env_mappings.items():
            env_value = os.getenv(env_var)
            if env_value is not None:
                # Navigate to the nested config location
                current = config
                for key in config_path[:-1]:
                    if key not in current:
                        current[key] = {}
                    current = current[key]
                
                # Convert value to appropriate type
                final_key = config_path[-1]
                current[final_key] = self._convert_env_value(env_value)
        
        return config
    
    def _convert_env_value(self, value: str):
        """Convert environment variable string to appropriate Python type."""
        # Boolean conversion
        if value.lower() in ('true', 'false'):
            return value.lower() == 'true'
        
        # Integer conversion
        try:
            return int(value)
        except ValueError:
            pass
        
        # Float conversion
        try:
            return float(value)
        except ValueError:
            pass
        
        # Return as string
        return value
    
    def get(self, *keys, default=None):
        """Get configuration value using dot notation.
        
        Args:
            *keys: Configuration keys (e.g., 'kafka', 'bootstrap_servers')
            default: Default value if key not found
            
        Returns:
            Configuration value or default
        """
        current = self._config
        for key in keys:
            if isinstance(current, dict) and key in current:
                current = current[key]
            else:
                return default
        return current
    
    @property
    def kafka(self) -> Dict[str, Any]:
        """Get Kafka configuration."""
        return self.get('kafka', default={})
    
    @property
    def mongodb(self) -> Dict[str, Any]:
        """Get MongoDB configuration."""
        return self.get('mongodb', default={})
    
    @property
    def fetcher(self) -> Dict[str, Any]:
        """Get HTTP fetcher configuration."""
        return self.get('fetcher', default={})
    
    @property
    def politeness(self) -> Dict[str, Any]:
        """Get politeness configuration."""
        return self.get('politeness', default={})
    
    @property
    def url_processing(self) -> Dict[str, Any]:
        """Get URL processing configuration."""
        return self.get('url_processing', default={})
    
    @property
    def crawler(self) -> Dict[str, Any]:
        """Get crawler behavior configuration."""
        return self.get('crawler', default={})
    
    @property
    def logging(self) -> Dict[str, Any]:
        """Get logging configuration."""
        return self.get('logging', default={})
    
    @property
    def metrics(self) -> Dict[str, Any]:
        """Get metrics configuration."""
        return self.get('metrics', default={})


# Global configuration instance
config = Config()