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
    filter: Optional[Union[Dict, FilterExpr]] = None
) -> FindResult:
    """Find resources using pure vector similarity without session context.

    Args:
        query: The semantic search query string.
        target_uri: The restricted directory prefix scope to search within.
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` directly executed by the DB.
    """
    client = OpenVK.get_client()

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
    filter: Optional[Union[Dict, FilterExpr]] = None
) -> FindResult:
    """Search resources with standard context.

    Args:
        query: The semantic search query string.
        target_uri: The restricted directory prefix scope to search within.
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` natively executed by the DB engine.
    """
    client = OpenVK.get_client()
    
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
    filter: Optional[Union[Dict, FilterExpr]] = None
) -> FindResult:
    """Perform a season-aware search heavily integrating conversation memory/messages as context.
    
    Args:
        query: The primary search query.
        msgs: List of previous conversation messages representing the session thread.
        target_uri: The restricted directory prefix scope to search within.
        limit: Maximum number of resources to return.
        score_threshold: Minimum vector similarity (0.0 to 1.0).
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr` natively executed by the DB engine.
    """
    client = OpenVK.get_client()
    
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


def read_resources_progressively(urls: List[str]) -> List[Dict[str, Any]]:
    """Read resource from OpenViking (resources scope only)

    Args:
        urls: List of resource URLs
        
    Returns:
        A list of dictionaries containing 'url' and the aggregated 'context'
    """
    client = OpenVK.get_client()
    
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


def read_resource(target: str, level: str = "L2") -> str:
    """Read resource from OpenViking (resources scope only)

    Args:
        target: Target URI
    """
    client = OpenVK.get_client()

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