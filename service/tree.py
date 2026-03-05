from typing import Dict, Any, List
from .client import OpenVK

def get_tree(uri: str) -> List[Dict[str, Any]]:
    """Get directory tree structure."""
    client = OpenVK.get_client()
    return client.tree(uri)

def grep_resources(uri: str, pattern: str, case_insensitive: bool = False) -> Dict[str, Any]:
    """Search content by pattern."""
    client = OpenVK.get_client()
    return client.grep(uri, pattern, case_insensitive=case_insensitive)

def glob_resources(pattern: str, uri: str = "viking://") -> Dict[str, Any]:
    """Match files by pattern."""
    client = OpenVK.get_client()
    return client.glob(pattern, uri)
