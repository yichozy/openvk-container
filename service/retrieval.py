from typing import Optional
from openviking.message import TextPart
from openviking.message import Message
from .client import OpenVK
from typing import Dict, Any, List, Union
from openviking_cli.retrieve.types import FindResult, MatchedContext

def find_resources(query: str,  target_uri: str = "", limit: int = 10, score_threshold: Optional[float] = None) -> FindResult:

    client = OpenVK.get_client()

    results = client.find(
        query,
        target_uri=target_uri,
        limit=limit,
        score_threshold=score_threshold,
    )

    return results

def search_resources(query: str, target_uri: str = "", limit: int = 10, score_threshold: Optional[float] = None, filter: Optional[Dict] = None) -> FindResult:
    """Search resources"""
    client = OpenVK.get_client()
    
    results = client.search(query, target_uri=target_uri, limit=limit, score_threshold=score_threshold, filter=filter)

    return results

def season_aware_search(query: str, msgs: List[Message], target_uri: str = "", limit: int = 10, score_threshold: Optional[float] = None, filter: Optional[Dict] = None) -> FindResult:
    """Season-aware search"""
    client = OpenVK.get_client()
    
    session = client.session()
    for msg in msgs:
        session.add_message(msg.role, [TextPart(msg.content)])

    results = client.search(query, session=session,target_uri=target_uri, limit=limit, score_threshold=score_threshold, filter=filter)

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