from typing import Dict, Any, Optional
from .client import OpenVK

def add_skill(data: Any, wait: bool = False, timeout: Optional[float] = None, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Add skill to OpenViking.

    Args:
        data: Skill data
        wait: Whether to wait
        timeout: Timeout in seconds
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.add_skill(data, wait=wait, timeout=timeout)
