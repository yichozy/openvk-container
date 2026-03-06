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


def read_resources_progressively(resources: List[MatchedContext]) -> str:
    """Read resource from OpenViking (resources scope only)

    Args:
        target: Target URI
    """
    client = OpenVK.get_client()
    
    ret_obj = ""
    
    for ctx in resources:
        # Get L0 (abstract)
        ret_obj += ctx.abstract

        if not ctx.is_leaf:
            # Get L1 (overview)
            overview = client.overview(ctx.uri)
            ret_obj += overview
        else:
            # Load L2 (content)
            content = client.read(ctx.uri)
            ret_obj += content

    # client.close()

    return ret_obj


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