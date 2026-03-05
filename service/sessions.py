from typing import Dict, Any, List, Optional
from .client import OpenVK

def session_exists(session_id: str) -> bool:
    """Check whether a session exists in storage."""
    client = OpenVK.get_client()
    return client.session_exists(session_id)

def create_session() -> Dict[str, Any]:
    """Create a new session."""
    client = OpenVK.get_client()
    return client.create_session()

def list_sessions() -> List[Any]:
    """List all sessions."""
    client = OpenVK.get_client()
    return client.list_sessions()

def get_session(session_id: str) -> Dict[str, Any]:
    """Get session details."""
    client = OpenVK.get_client()
    return client.get_session(session_id)

def delete_session(session_id: str) -> None:
    """Delete a session."""
    client = OpenVK.get_client()
    client.delete_session(session_id)

def add_message(session_id: str, role: str, content: Optional[str] = None, parts: Optional[List[Dict]] = None) -> Dict[str, Any]:
    """Add a message to a session."""
    client = OpenVK.get_client()
    return client.add_message(session_id, role, content=content, parts=parts)

def commit_session(session_id: str) -> Dict[str, Any]:
    """Commit a session."""
    client = OpenVK.get_client()
    return client.commit_session(session_id)
