from litellm import PerplexityResponsesConfig
from typing import Optional, Dict, Any, List, Union
from openviking.message import TextPart
from openviking.message import Message
from .client import OpenVK
from openviking_cli.retrieve.types import FindResult, MatchedContext
from openviking.storage.expr import FilterExpr
import heapq


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


def recursive_search(
    query: str,
    target_uri: str = "",
    topK: int = 3,
    score_threshold: Optional[float] = None,
    filter: Optional[Union[Dict, FilterExpr]] = None,
    max_rounds: int = 3,
    context_type: Optional[str] = None,
    max_relations: int = 3,
    tenant_id: str = "workspace"
) -> FindResult:
    """Perform a recursive search propagating scores across directories.

    Args:
        query: The semantic search query string.
        target_uri: The restricted directory prefix scope to search within.
        topK: Maximum number of resources to return.
        score_threshold: Minimum final propagation score.
        filter: An advanced JSON AST raw dictionary or an `openviking.storage.expr.FilterExpr`.
        max_rounds: Maximum expansion rounds to prevent infinite loops.
        context_type: Type of context for mapping root directories (MEMORY, RESOURCE, SKILL).
        max_relations: Maximum number of relations to retain per resource.
        tenant_id: Tenant ID for multi-tenancy support
    """
    client = OpenVK.get_client(tenant_id=tenant_id)
    import posixpath
    
    # Key Parameters
    SCORE_PROPAGATION_ALPHA = 0.5
    
    dir_queue = []
    
    if target_uri:
        is_leaf = True
        try:
            stat_info = client.stat(target_uri)
            is_leaf = stat_info.get("is_leaf", False) if isinstance(stat_info, dict) else getattr(stat_info, "is_leaf", False)
        except Exception:
            pass
            
        if not is_leaf:
            heapq.heappush(dir_queue, (-1.0, target_uri))
    else:
        root_uris = []
        if context_type == "MEMORY":
            root_uris = ["viking://user/memories", "viking://agent/memories"]
        elif context_type == "SKILL":
            root_uris = ["viking://agent/skills"]
        elif context_type == "RESOURCE":
            root_uris = ["viking://resources"]
        else:
            root_uris = ["viking://resources"]
            
        for r_uri in root_uris:
            heapq.heappush(dir_queue, (-1.0, r_uri))

    collected: List[MatchedContext] = []
    topk_unchanged_rounds = 0
    last_topk_uris = []
    rounds = 0
    threshold = score_threshold if score_threshold is not None else 0.0

    if not dir_queue and target_uri:
        try:
            results = client.search(
                query, 
                target_uri=target_uri, 
                limit=topK, 
                score_threshold=score_threshold,  
                filter=filter
            )
            found_items = results.resources if hasattr(results, 'resources') and results.resources else []
            collected.extend(found_items)
        except Exception:
            pass
    else:
        while dir_queue and rounds < max_rounds:
            rounds += 1
            neg_parent_score, current_uri = heapq.heappop(dir_queue)
            parent_score = -neg_parent_score
            
            try:
                results = client.search(
                    query, 
                    target_uri=current_uri, 
                    limit=topK, 
                    score_threshold=None,  
                    filter=filter
                )
                found_items = results.resources if hasattr(results, 'resources') and results.resources else []
            except Exception:
                found_items = []
                
            for r in found_items:
                embedding_score = r.score
                
                # Score propagation
                final_score = SCORE_PROPAGATION_ALPHA * embedding_score + (1.0 - SCORE_PROPAGATION_ALPHA) * parent_score
                
                if final_score > threshold:
                    r.score = final_score

                    is_leaf = r.level == 2
                    collected.append(r)

                    if not is_leaf:  # Directory continues recursion
                        heapq.heappush(dir_queue, (-final_score, r.uri))
                        
            # Convergence detection
            collected.sort(key=lambda x: x.score, reverse=True)
            deduped = []
            seen = set()
            for c in collected:
                if c.uri not in seen:
                    deduped.append(c)
                    seen.add(c.uri)
                    
            current_topk_uris = [c.uri for c in deduped[:topK]]
            
            if current_topk_uris == last_topk_uris and len(current_topk_uris) > 0:
                topk_unchanged_rounds += 1
            else:
                topk_unchanged_rounds = 0
                last_topk_uris = current_topk_uris
                
            if topk_unchanged_rounds >= 3:
                break
                
    # Step 5: Convert to MatchedContext
    unique_collected = []
    seen_uris = set()
    for c in collected:
        if c.uri not in seen_uris:
            unique_collected.append(c)
            seen_uris.add(c.uri)
            
    final_results = unique_collected
    
    for r in final_results:
        # Limit max relations parameters if field exists
        if hasattr(r, "relations") and r.relations is not None:
            r.relations = r.relations[:max_relations]
            
    return FindResult(
        resources=final_results,
        memories=[],
        skills=[],
        query_plan=None,
        query_results=None,
        total=len(final_results)
    )
