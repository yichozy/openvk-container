from litellm import PerplexityResponsesConfig
from typing import Optional, Dict, Any, List, Union
from openviking.message import TextPart
from openviking.message import Message
from .client import OpenVK
from openviking_cli.retrieve.types import FindResult, MatchedContext
from openviking.storage.expr import FilterExpr


def find_resources(
    query: str,
    target_uri: str = "",
    limit: int = 10,
    score_threshold: Optional[float] = None,
    filter: Optional[Union[Dict, FilterExpr]] = None,
    tenant_id: str = "workspace"
) -> FindResult:
    """Find resources using pure vector similarity without session context.

    Args:
        query: The semantic search query string.
        target_uri: The restricted directory prefix scope to search within.
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` directly executed by the DB.
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)

    results = client.find(
        query,
        target_uri=target_uri,
        limit=limit,
        score_threshold=score_threshold,
        filter=filter,
    )

    return results

def search_resources(
    query: str,
    target_uri: str = "",
    limit: int = 10,
    score_threshold: Optional[float] = None,
    filter: Optional[Union[Dict, FilterExpr]] = None,
    tenant_id: str = "workspace"
) -> FindResult:
    """Search resources with standard context.

    Args:
        query: The semantic search query string.
        target_uri: The restricted directory prefix scope to search within.
        tenant_id: Tenant ID for multi-tenancy support
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` natively executed by the DB engine.
    """
    client = OpenVK.get_client(tenant_id=tenant_id)

    results = client.search(
        query,
        target_uri=target_uri,
        limit=limit,
        score_threshold=score_threshold,
        filter=filter
    )

    return results

def season_aware_search(
    query: str,
    msgs: List[Message],
    target_uri: str = "",
    limit: int = 10,
    score_threshold: Optional[float] = None,
    filter: Optional[Union[Dict, FilterExpr]] = None,
    tenant_id: str = "workspace"
) -> FindResult:
    """Perform a season-aware search heavily integrating conversation memory/messages as context.

    Args:
        query: The primary search query.
        msgs: List of previous conversation messages representing the session thread.
        target_uri: The restricted directory prefix scope to search within.
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` natively executed by the DB engine.
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    
    session = client.session()
    for msg in msgs:
        session.add_message(msg.role, [TextPart(msg.content)])

    results = client.search(
        query, 
        session=session,
        target_uri=target_uri, 
        limit=limit, 
        score_threshold=score_threshold, 
        filter=filter
    )

    return results


def read_resources_progressively(urls: List[str], tenant_id: str = "workspace") -> List[Dict[str, Any]]:
    """Read resource from OpenViking (resources scope only)

    Args:
        urls: List of resource URLs
        tenant_id: Tenant ID for multi-tenancy support

    Returns:
        A list of dictionaries containing 'url' and the aggregated 'context'
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    
    resource_docs = []
    
    for url in urls:

        ret_obj = []

        try:
            stat_info = client.stat(url)
            # handle cases where stat returns a dict or object
            is_leaf = stat_info.get("is_leaf", False) if isinstance(stat_info, dict) else getattr(stat_info, "is_leaf", False)

            # Get L0 (abstract)
            abstract = client.abstract(url)
            if abstract:
                ret_obj.append("Abstract: " + abstract)

            if not is_leaf:
                # Get L1 (overview)
                overview = client.overview(url)
                if overview:
                    ret_obj.append("Overview: " + overview)
            else:
                # Load L2 (content)
                content = client.read(url)
                if content:
                    ret_obj.append("File content: " +content)
        except Exception as e:
            pass

        if len(ret_obj) > 0 :

            contents = "\n\n".join(ret_obj)

            resource_docs.append({
                "url": url,
                "context": contents
            })

    return resource_docs


def read_resource(target: str, level: str = "L2", tenant_id: str = "workspace") -> str:
    """Read resource from OpenViking (resources scope only)

    Args:
        target: Target URI
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)

    ret_obj = ""

    if level == "L0":
        ret_obj = client.abstract(target)
    elif level == "L1":
        ret_obj = client.overview(target)
    elif level == "L2":
        ret_obj = client.read(target)
    else:
        raise ValueError("Invalid level. Must be L0, L1, or L2")

    # client.close()

    return ret_obj


def read_resource_bytes(target: str, tenant_id: str = "workspace") -> bytes:
    """Read resource as raw bytes (for binary files like images).

    Uses viking_fs.read_file_bytes() to avoid UTF-8 corruption
    that occurs with the normal read() path.

    Args:
        target: Target URI
        tenant_id: Tenant ID for multi-tenancy support

    Returns:
        Raw bytes of the file content
    """
    from openviking_cli.utils import run_async

    client = OpenVK.get_client(tenant_id=tenant_id)
    fs_service = client._async_client._client.service.fs
    return run_async(fs_service.read_file_bytes(target, ctx=None))
