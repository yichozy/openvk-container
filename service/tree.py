from typing import Dict, Any, List
from .client import OpenVK

def get_tree(uri: str, tenant_id: str = "workspace") -> List[Dict[str, Any]]:
    """Get directory tree structure.

    Args:
        uri: Target URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.tree(uri)

def grep_resources(uri: str, pattern: str, case_insensitive: bool = False, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Search content by pattern.

    Args:
        uri: Target URI
        pattern: Search pattern
        case_insensitive: Whether to ignore case
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.grep(uri, pattern, case_insensitive=case_insensitive)

def glob_resources(pattern: str, uri: str = "viking://", tenant_id: str = "workspace") -> Dict[str, Any]:
    """Match files by pattern.

    Args:
        pattern: Glob pattern
        uri: Target URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.glob(pattern, uri)
