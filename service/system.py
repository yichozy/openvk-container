from typing import Dict, Any
from .client import OpenVK

def get_status(tenant_id: str = "workspace") -> Any:
    """Get system status.

    Args:
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.get_status()

def is_healthy(tenant_id: str = "workspace") -> bool:
    """Quick health check.

    Args:
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.is_healthy()
