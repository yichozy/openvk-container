from .client import OpenVK
from typing import Dict, Any, List, Union, Optional
import os
from urllib.parse import urlparse, unquote
import tempfile
import requests
import shutil
from openviking_cli.utils import run_async

def add_resource(path_or_url: str, to: str = None, parent: str = None, reason: str = "", replace: bool = False, instruction: str = "", wait: bool = True, timeout: Optional[float] = None, build_index: bool = True, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Add resource to OpenViking (resources scope only)

    Args:
        path_or_url: Path or URL to add
        to: Exact target URI (must not exist yet)
        parent: Target parent URI (must already exist)
        reason: Reason for adding
        replace: Whether to remove the old resource before adding
        instruction: Instruction for adding
        wait: Whether to wait for async operations to complete
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    
    if to and parent:
        raise ValueError("Cannot specify both 'to' and 'parent' at the same time.")
    if not to and not parent:
        raise ValueError("Must specify either 'to' or 'parent'.")

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

    if to:
        file_url = to
    else:
        name_only = os.path.splitext(filename)[0]
        file_url = os.path.join(parent, name_only)

    is_file_existed = False
    is_replaced = False

    try:
        client.stat(file_url)
        is_file_existed = True
    except Exception as e:
        # Resource might not exist, proceed
        print(f"Resource {file_url} not found: {e}")
    
    # if replace = True, remove the old resource
    if is_file_existed and replace: 
        client.rm(file_url, recursive=True)
        is_replaced = True

    elif is_file_existed:
        return {"is_replaced": is_replaced, "msg": "Resource already exists"}

     # Option 1: Wait inline
    result = client.add_resource(
        tmp_path_or_url,
        to=to,
        parent=parent,
        reason=reason,
        instruction=instruction,
        wait=wait,
        timeout=timeout,
        build_index=build_index,
    )

    print(result)

    try:
        if tmp_dir is not None:
            os.remove(tmp_path_or_url)
    finally:
        pass
            
    return {"is_replaced": is_replaced, "msg": "Resource added successfully", "result": result}

    

def replace_resource(path_or_url: str, to: str = None, parent: str = None, reason: str = "", instruction: str = "", wait: bool = True, timeout: Optional[float] = None, build_index: bool = True, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Replace resource in OpenViking

    Args:
        path_or_url: Path or URL for replacement
        to: Exact target URI to replace
        parent: Target parent URI
        reason: Reason for replacing
        instruction: Instruction for replacing
        wait: Whether to wait for async operations to complete
        timeout: Wait timeout in seconds
        build_index: Whether to build vector index immediately
        tenant_id: Tenant ID for multi-tenancy support
    """
    return add_resource(path_or_url, to=to, parent=parent, reason=reason, replace=True, instruction=instruction, wait=wait, timeout=timeout, build_index=build_index, tenant_id=tenant_id)

def list_resources(target: str, simple: bool = False, recursive: bool = False, tenant_id: str = "workspace") -> List[Any]:
    """List resources in OpenViking (resources scope only)

    Args:
        target: Target URI
        simple: If True, returns a list of path strings instead of detailed objects.
        recursive: If True, recursively lists all inner paths and files.
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    
    resources = client.ls(target, simple=simple, recursive=recursive)

    # client.close()

    return resources



def get_resource_relations(target: str, tenant_id: str = "workspace") -> List[Any]:
    """Get relations for a resource

    Args:
        target: Target resource URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    relations = client.relations(target)
    # client.close()
    return relations

def move_resource(src: str, dest: str, tenant_id: str = "workspace") -> None:
    """Move resources from src to dest

    Args:
        src: Source URI
        dest: Destination URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.mv(src, dest)
    # client.close()

def delete_resource(target: str, recursive: bool = False, tenant_id: str = "workspace") -> None:
    """Delete resources

    Args:
        target: Target resource URI
        recursive: Whether to delete recursively
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.rm(target, recursive=recursive)
    # client.close()

def link_resources(src: str, dest: Union[str, List[str]], reason: str = "", tenant_id: str = "workspace") -> None:
    """Create Links between resources

    Args:
        src: Source URI
        dest: Destination URI(s)
        reason: Reason for linking
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.link(src, dest, reason=reason)
    # client.close()

def get_relations(target: str, tenant_id: str = "workspace") -> List[Any]:
    """Get relations for a resource

    Args:
        target: Target resource URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    relations = client.relations(target)
    # client.close()
    return relations

def unlink_resources(src: str, dest: str, tenant_id: str = "workspace") -> None:
    """Remove Links between resources

    Args:
        src: Source URI
        dest: Destination URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.unlink(src, dest)
    # client.close()

def export_ovpack(target: str, to: str, tenant_id: str = "workspace") -> str:
    """Export .ovpack file

    Args:
        target: Target URI to export
        to: Output file path
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.export_ovpack(target, to)

def import_ovpack(file_path: str, target: str, force: bool = False, vectorize: bool = True, tenant_id: str = "workspace") -> str:
    """Import .ovpack file

    Args:
        file_path: Path to .ovpack file
        target: Target URI to import to
        force: Whether to force overwrite
        vectorize: Whether to run vectorization
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.import_ovpack(file_path, target, force, vectorize)

def stat(uri: str, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Get resource status

    Args:
        uri: Resource URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.stat(uri)

def mkdir(uri: str, tenant_id: str = "workspace") -> None:
    """Create directory

    Args:
        uri: Directory URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    client.mkdir(uri)

def wait_processed(timeout: float = None, tenant_id: str = "workspace") -> Dict[str, Any]:
    """Wait for all async operations to complete

    Args:
        timeout: Timeout in seconds
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    return client.wait_processed(timeout)

def build_index(resource_uris: Union[str, List[str]], tenant_id: str = "workspace", **kwargs) -> Dict[str, Any]:
    """Manually trigger index building.

    Args:
        resource_uris: Resource URIs to build index for
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    if hasattr(client, "build_index"):
        return client.build_index(resource_uris, **kwargs)
    else:
        return run_async(client._async_client.build_index(resource_uris, **kwargs))
