from .client import OpenVK
from typing import Dict, Any, List, Union

def add_resource(path_or_url: str, target: str, reason: str = "") -> Dict[str, Any]:
    """Add resource to OpenViking (resources scope only)

    Args:
        path_or_url: Path or URL to add
        target: Target URI
        reason: Reason for adding
    """
    client = OpenVK.get_client()
    
    # Option 1: Wait inline
    client.add_resource(
        path_or_url,
        target=target,
        reason=reason,
    )

    status = client.wait_processed()


    return status

def list_resources(target: str, simple: bool = False, recursive: bool = False) -> List[Any]:
    """List resources in OpenViking (resources scope only)

    Args:
        target: Target URI
        simple: If True, returns a list of path strings instead of detailed objects.
        recursive: If True, recursively lists all inner paths and files.
    """
    client = OpenVK.get_client()
    
    resources = client.ls(target, simple=simple, recursive=recursive)

    # client.close()

    return resources



def get_resource_relations(target: str) -> List[Any]:
    """Get relations for a resource"""
    client = OpenVK.get_client()
    relations = client.relations(target)
    # client.close()
    return relations

def move_resource(src: str, dest: str) -> None:
    """Move resources from src to dest"""
    client = OpenVK.get_client()
    client.mv(src, dest)
    # client.close()

def delete_resource(target: str, recursive: bool = False) -> None:
    """Delete resources"""
    client = OpenVK.get_client()
    client.rm(target, recursive=recursive)
    # client.close()

def link_resources(src: str, dest: Union[str, List[str]], reason: str = "") -> None:
    """Create Links between resources"""
    client = OpenVK.get_client()
    client.link(src, dest, reason=reason)
    # client.close()

def get_relations(target: str) -> List[Any]:
    """Get relations for a resource"""
    client = OpenVK.get_client()
    relations = client.relations(target)
    # client.close()
    return relations

def unlink_resources(src: str, dest: str) -> None:
    """Remove Links between resources"""
    client = OpenVK.get_client()
    client.unlink(src, dest)
    # client.close()

def export_ovpack(target: str, to: str) -> str:
    """Export .ovpack file"""
    client = OpenVK.get_client()
    return client.export_ovpack(target, to)

def import_ovpack(file_path: str, target: str, force: bool = False, vectorize: bool = True) -> str:
    """Import .ovpack file"""
    client = OpenVK.get_client()
    return client.import_ovpack(file_path, target, force, vectorize)

def stat(uri: str) -> Dict[str, Any]:
    """Get resource status"""
    client = OpenVK.get_client()
    return client.stat(uri)

def mkdir(uri: str) -> None:
    """Create directory"""
    client = OpenVK.get_client()
    client.mkdir(uri)

def wait_processed(timeout: float = None) -> Dict[str, Any]:
    """Wait for all async operations to complete"""
    client = OpenVK.get_client()
    return client.wait_processed(timeout)


