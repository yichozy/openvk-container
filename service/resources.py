from .client import OpenVK
from typing import Dict, Any, List, Union
import os
from urllib.parse import urlparse, unquote
import tempfile
import requests
import shutil

def add_resource(path_or_url: str, target: str, reason: str = "", replace: bool = False) -> Dict[str, Any]:
    """Add resource to OpenViking (resources scope only)

    Args:
        path_or_url: Path or URL to add
        target: Target URI
        reason: Reason for adding
        replace: Whether to remove the old resource before adding
    """
    client = OpenVK.get_client()

    tmp_path_or_url = path_or_url

    tmp_dir = None

    if tmp_path_or_url.startswith("http"):
        # decode tmp_path_or_url
        decoded_url = unquote(tmp_path_or_url)
        path = urlparse(decoded_url).path
        filename = os.path.basename(path)

        response = requests.get(tmp_path_or_url, stream=True)
        response.raise_for_status()
        tmp_dir = tempfile.mkdtemp()
        # If the filename from the URL is empty or lacks an extension, fallback to a default
        actual_filename = filename if filename else "downloaded_file"
        tmp_path_or_url = os.path.join(tmp_dir, actual_filename)
        
        with open(tmp_path_or_url, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    f.write(chunk)
        


    else:
        filename = os.path.basename(tmp_path_or_url)

    name_only = os.path.splitext(filename)[0]
    file_url = os.path.join(target, name_only)

    is_file_existed = False
    is_replaced = False

    try:
        client.stat(file_url)
        is_file_existed = True
    except Exception as e:
        # Resource might not exist, proceed
        print(f"Resource {target} not found: {e}")
    
    # if replace = True, remove the old resource
    if is_file_existed and replace: 
        client.rm(file_url, recursive=True)
        is_replaced = True

    elif is_file_existed:
        return {"is_replaced": is_replaced, "msg": "Resource already exists"}

     # Option 1: Wait inline
    result = client.add_resource(
        tmp_path_or_url,
        target=target,
        reason=reason,
    )

    print(result)

    try:
        if tmp_dir is not None:
            os.remove(tmp_path_or_url)
    finally:
        pass
            
    return {"is_replaced": is_replaced, "msg": "Resource added successfully", "result": result}

    

def replace_resource(path_or_url: str, target: str, reason: str = "") -> Dict[str, Any]:
    """Replace resource in OpenViking

    Args:
        path_or_url: Path or URL for replacement
        target: Target URI to replace
        reason: Reason for replacing
    """
    return add_resource(path_or_url, target, reason=reason, replace=True)

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


