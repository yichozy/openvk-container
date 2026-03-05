from typing import Dict, Any
from .client import OpenVK

def get_status() -> Any:
    """Get system status."""
    client = OpenVK.get_client()
    return client.get_status()

def is_healthy() -> bool:
    """Quick health check."""
    client = OpenVK.get_client()
    return client.is_healthy()
