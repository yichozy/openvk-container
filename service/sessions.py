from typing import Dict, Any, List, Optional
from .client import OpenVK

def session_exists(session_id: str, tenant_id: str = "workspace") -> bool:
    """Check whether a session exists in storage.

    Args:
        session_id: Session ID
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.session_exists(session_id)

def create_session(tenant_id: str = "workspace") -> Dict[str, Any]:
    """Create a new session.

    Args:
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.create_session()

def list_sessions(tenant_id: str = "workspace") -> List[Any]:
    """List all sessions.

    Args:
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.list_sessions()

def get_session(session_id: str, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Get session details.

    Args:
        session_id: Session ID
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.get_session(session_id)

def delete_session(session_id: str, tenant_id: str = "workspace") -> None:
    """Delete a session.

    Args:
        session_id: Session ID
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.delete_session(session_id)

def add_message(session_id: str, role: str, content: Optional[str] = None, parts: Optional[List[Dict]] = None, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Add a message to a session.

    Args:
        session_id: Session ID
        role: Role (user/assistant)
        content: Message content
        parts: Message parts
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.add_message(session_id, role, content=content, parts=parts)

def commit_session(session_id: str, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Commit a session.

    Args:
        session_id: Session ID
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.commit_session(session_id)
