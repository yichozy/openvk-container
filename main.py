from contextlib import asynccontextmanager
from typing import List, Union, Dict, Any, Optional

from fastapi import FastAPI, HTTPException, UploadFile, File, Form, BackgroundTasks, Request
from fastapi.responses import FileResponse, Response
from pydantic import BaseModel, Field
import tempfile
import shutil
import os

from openviking.message import Message
from service.resources import (
    add_resource,
    replace_resource,
    list_resources,
    get_resource_relations,
    move_resource,
    delete_resource,
    link_resources,
    unlink_resources,
    export_ovpack,
    import_ovpack,
    stat,
    mkdir,
    wait_processed,
    build_index
)
from service.retrieval import (
    find_resources,
    season_aware_search,
    search_resources,
    recursive_search,
    read_resources_progressively,
    read_resource
)
from service.tree import (
    get_tree,
    grep_resources,
    glob_resources
)
from service.sessions import (
    session_exists, create_session, list_sessions, get_session,
    delete_session, add_message, commit_session
)
from service.skills import add_skill
from service.system import get_status, is_healthy

@asynccontextmanager
async def lifespan(app: FastAPI):
    yield
    from service.client import OpenVK
    OpenVK.close_client()

app = FastAPI(title="OpenViking Client API", lifespan=lifespan)

# --- Pydantic Models for Resources ---
class ExportOvpackRequest(BaseModel):
    uri: str = Field(..., description="Viking URI to export from")

class MkdirRequest(BaseModel):
    uri: str = Field(..., description="URI to create directory for")

class BuildIndexRequest(BaseModel):
    resource_uris: Union[str, List[str]] = Field(..., description="Resource URIs to build index for")

class AddMessageRequest(BaseModel):
    role: str = Field(..., description="Role ('user' or 'assistant')")
    content: Optional[str] = None
    parts: Optional[List[Dict]] = None
    

class AddResourceURLRequest(BaseModel):
    url: str = Field(..., description="URL to add")
    to: Optional[str] = Field(None, description="Exact target URI (must not exist yet)")
    parent: Optional[str] = Field(None, description="Target parent URI (must already exist)")
    reason: str = Field("", description="Reason for adding the resource")
    replace: bool = Field(False, description="Whether to remove the old resource before adding")
    instruction: str = Field("", description="Instruction for processing the resource")
    wait: bool = Field(True, description="Whether to wait for async operations to complete")
    timeout: Optional[float] = Field(None, description="Wait timeout in seconds")
    build_index: bool = Field(True, description="Whether to build vector index immediately")

class ReplaceResourceURLRequest(BaseModel):
    url: str = Field(..., description="URL for replacement")
    to: Optional[str] = Field(None, description="Exact target URI (must not exist yet)")
    parent: Optional[str] = Field(None, description="Target parent URI (must already exist)")
    reason: str = Field("", description="Reason for replacing the resource")
    instruction: str = Field("", description="Instruction for replacing the resource")
    wait: bool = Field(True, description="Whether to wait for async operations to complete")
    timeout: Optional[float] = Field(None, description="Wait timeout in seconds")
    build_index: bool = Field(True, description="Whether to build vector index immediately")

class GrepResourceRequest(BaseModel):
    uri: str = Field(..., description="Viking URI to search in")
    pattern: str = Field(..., description="Search pattern (regex)")
    case_insensitive: bool = Field(False, description="Ignore case")

class GlobResourceRequest(BaseModel):
    pattern: str = Field(..., description="Glob pattern (e.g., **/*.md)")
    uri: str = Field("viking://", description="Starting URI")

class MoveResourceRequest(BaseModel):
    src: str = Field(..., description="Source resource URI")
    dest: str = Field(..., description="Destination resource URI")

class DeleteResourceRequest(BaseModel):
    target: str = Field(..., description="Resource URI to delete")
    recursive: bool = Field(False, description="Whether to recursively delete a directory")

class LinkResourceRequest(BaseModel):
    src: str = Field(..., description="Source resource URI")
    dest: Union[str, List[str]] = Field(..., description="Destination resource URI(s)")
    reason: str = Field("", description="Reason for linking")

class UnlinkResourceRequest(BaseModel):
    src: str = Field(..., description="Source resource URI")
    dest: str = Field(..., description="Destination resource URI to unlink")


# --- Pydantic Models for Retrieval ---

class FindRequest(BaseModel):
    query: str = Field(..., description="Search query string")
    target_uri: str = Field("", description="Optional target URI context")
    limit: int = Field(10, description="Limit on number of results")
    score_threshold: Optional[float] = Field(None, description="Score threshold for filtering")
    filter: Optional[Dict] = Field(None, description="Optional JSON AST filter (e.g., {'op': 'and', 'conds': [...]})")

class MessageItem(BaseModel):
    role: str = Field(..., description="Role of the sender ('user' or 'assistant')")
    content: str = Field(..., description="Message content")

class SearchRequest(BaseModel):
    query: str = Field(..., description="Search query string")
    target_uri: str = Field("", description="Optional target URI context")
    limit: int = Field(10, description="Limit on number of results")
    msgs: Optional[List[MessageItem]] = Field([],description="List of previous conversation messages")
    score_threshold: Optional[float] = Field(None, description="Score threshold for filtering")
    filter: Optional[Dict] = Field(None, description="Optional filter dict")

class RecursiveSearchRequest(BaseModel):
    query: str = Field(..., description="Search query string")
    target_uri: str = Field("", description="Optional target URI context")
    limit: int = Field(10, description="Limit on number of results")
    score_threshold: Optional[float] = Field(None, description="Score threshold for filtering")
    filter: Optional[Dict] = Field(None, description="Optional filter dict")
    max_rounds: int = Field(10, description="Maximum expansion rounds")

class ReadProgressivelyRequest(BaseModel):
    urls: List[str] = Field(..., description="List of resource URLs to read context from")

class ReadProgressivelyResponseItem(BaseModel):
    url: str = Field(..., description="The URL of the resource")
    context: str = Field(..., description="The concatenated context read (abstract, overview, and content)")

class ReadProgressivelyResponse(BaseModel):
    status: str = Field(..., description="Status of the operation")
    data: List[ReadProgressivelyResponseItem] = Field(..., description="List of processed resource items")


# ==========================================
# Routes: Resources
# ==========================================

@app.post("/resources/add_url", summary="Add a Resource via URL")
def api_add_resource_url(req: AddResourceURLRequest):
    try:
        status = add_resource(path_or_url=req.url, to=req.to, parent=req.parent, reason=req.reason, replace=req.replace, instruction=req.instruction, wait=req.wait, timeout=req.timeout, build_index=req.build_index)
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/add_file", summary="Add a Resource via File Upload")
def api_add_resource_file(
    file: UploadFile = File(..., description="Uploaded file to add"),
    to: Optional[str] = Form(None, description="Exact target URI (must not exist yet)"),
    parent: Optional[str] = Form(None, description="Target parent URI (must already exist)"),
    reason: str = Form("", description="Reason for adding the resource"),
    replace: bool = Form(False, description="Whether to remove the old resource before adding"),
    instruction: str = Form("", description="Instruction for processing the resource"),
    wait: bool = Form(True, description="Whether to wait for async operations to complete"),
    timeout: Optional[float] = Form(None, description="Wait timeout in seconds"),
    build_index: bool = Form(True, description="Whether to build vector index immediately")
):
    try:
        # Create a temp directory
        temp_dir = tempfile.mkdtemp()
        temp_file_path = os.path.join(temp_dir, file.filename)
        
        with open(temp_file_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)
            
        status = add_resource(path_or_url=temp_file_path, to=to, parent=parent, reason=reason, replace=replace, instruction=instruction, wait=wait, timeout=timeout, build_index=build_index)
        
        # Attempt to clean up
        try:
            os.remove(temp_file_path)
            os.rmdir(temp_dir)
        except Exception:
            pass
            
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/replace_url", summary="Replace a Resource via URL")
def api_replace_resource_url(req: ReplaceResourceURLRequest):
    try:
        status = replace_resource(path_or_url=req.url, to=req.to, parent=req.parent, reason=req.reason, instruction=req.instruction, wait=req.wait, timeout=req.timeout, build_index=req.build_index)
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/replace_file", summary="Replace a Resource via File Upload")
def api_replace_resource_file(
    file: UploadFile = File(..., description="Uploaded file for replacement"),
    to: Optional[str] = Form(None, description="Exact target URI (must not exist yet)"),
    parent: Optional[str] = Form(None, description="Target parent URI (must already exist)"),
    reason: str = Form("", description="Reason for replacing the resource"),
    instruction: str = Form("", description="Instruction for replacing the resource"),
    wait: bool = Form(True, description="Whether to wait for async operations to complete"),
    timeout: Optional[float] = Form(None, description="Wait timeout in seconds"),
    build_index: bool = Form(True, description="Whether to build vector index immediately")
):
    try:
        # Create a temp directory
        temp_dir = tempfile.mkdtemp()
        temp_file_path = os.path.join(temp_dir, file.filename)
        
        with open(temp_file_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)
            
        status = replace_resource(path_or_url=temp_file_path, to=to, parent=parent, reason=reason, instruction=instruction, wait=wait, timeout=timeout, build_index=build_index)
        
        # Attempt to clean up
        try:
            os.remove(temp_file_path)
            os.rmdir(temp_dir)
        except Exception:
            pass
            
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/add_bytes", summary="Add a Resource via Raw Bytes")
async def api_add_resource_bytes(
    request: Request,
    filename: str,
    to: Optional[str] = None,
    parent: Optional[str] = None,
    reason: str = "",
    replace: bool = False,
    instruction: str = "",
    wait: bool = True,
    timeout: Optional[float] = None,
    build_index: bool = True
):
    try:
        body_bytes = await request.body()
        
        # Create a temp directory
        temp_dir = tempfile.mkdtemp()
        temp_file_path = os.path.join(temp_dir, filename)
        
        with open(temp_file_path, "wb") as buffer:
            buffer.write(body_bytes)
            
        status = add_resource(path_or_url=temp_file_path, to=to, parent=parent, reason=reason, replace=replace, instruction=instruction, wait=wait, timeout=timeout, build_index=build_index)
        
        # Attempt to clean up
        try:
            os.remove(temp_file_path)
            os.rmdir(temp_dir)
        except Exception:
            pass
            
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/resources/list", summary="List Resources")
def api_list_resources(target: str, simple: bool = False, recursive: bool = False):
    try:
        resources = list_resources(target, simple=simple, recursive=recursive)
        return {"status": "success", "data": resources}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/resources/relations", summary="Get Resource Relations")
def api_get_relations(target: str):
    try:
        relations = get_resource_relations(target)
        return {"status": "success", "data": relations}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/move", summary="Move a Resource")
def api_move_resource(req: MoveResourceRequest):
    try:
        move_resource(src=req.src, dest=req.dest)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/resources/delete", summary="Delete a Resource")
def api_delete_resource(req: DeleteResourceRequest):
    try:
        delete_resource(target=req.target, recursive=req.recursive)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/link", summary="Link Resources")
def api_link_resources(req: LinkResourceRequest):
    try:
        link_resources(src=req.src, dest=req.dest, reason=req.reason)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/unlink", summary="Unlink Resources")
def api_unlink_resources(req: UnlinkResourceRequest):
    try:
        unlink_resources(src=req.src, dest=req.dest)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/resources/tree", summary="Get Resource Tree")
def api_get_tree(target: str):
    try:
        tree_data = get_tree(target)
        return {"status": "success", "data": tree_data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/grep", summary="Grep in Resources")
def api_grep_resources(req: GrepResourceRequest):
    try:
        results = grep_resources(uri=req.uri, pattern=req.pattern, case_insensitive=req.case_insensitive)
        return {"status": "success", "data": results}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/glob", summary="Glob Match Resources")
def api_glob_resources(req: GlobResourceRequest):
    try:
        results = glob_resources(pattern=req.pattern, uri=req.uri)
        return {"status": "success", "data": results}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# ==========================================
# Routes: Retrieval
# ==========================================

@app.post("/retrieval/find", summary="Find Resources")
def api_find_resources(req: FindRequest):
    try:
        results = find_resources(
            query=req.query, 
            target_uri=req.target_uri, 
            limit=req.limit, 
            score_threshold=req.score_threshold,
            filter=req.filter
        )
        
        # safely convert to dict
        if hasattr(results, "to_dict"):
            res_data = results.to_dict()
            if hasattr(results, "query_results") and results.query_results:
                res_data["query_results"] = [
                    {
                        "query": results._query_to_dict(qr.query) if hasattr(results, "_query_to_dict") else str(qr.query),
                        "matched_contexts": [results._context_to_dict(c) if hasattr(results, "_context_to_dict") else c.uri for c in qr.matched_contexts],
                        "searched_directories": qr.searched_directories,
                        "thinking_trace": qr.thinking_trace.to_dict() if hasattr(qr.thinking_trace, "to_dict") else {}
                    }
                    for qr in results.query_results
                ]
        else:
            res_data = results
            
        return {"status": "success", "data": res_data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrieval/recursive_search", summary="Recursive Search Resources")
def api_recursive_search(req: RecursiveSearchRequest):
    try:
        results = recursive_search(
            query=req.query, 
            target_uri=req.target_uri, 
            limit=req.limit, 
            score_threshold=req.score_threshold,
            filter=req.filter,
            max_rounds=req.max_rounds
        )
        
        # safely convert to dict
        if hasattr(results, "to_dict"):
            res_data = results.to_dict()
            if hasattr(results, "query_results") and results.query_results:
                res_data["query_results"] = [
                    {
                        "query": results._query_to_dict(qr.query) if hasattr(results, "_query_to_dict") else str(qr.query),
                        "matched_contexts": [results._context_to_dict(c) if hasattr(results, "_context_to_dict") else c.uri for c in qr.matched_contexts],
                        "searched_directories": qr.searched_directories,
                        "thinking_trace": qr.thinking_trace.to_dict() if hasattr(qr.thinking_trace, "to_dict") else {}
                    }
                    for qr in results.query_results
                ]
        else:
            res_data = results
            
        return {"status": "success", "data": res_data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrieval/search", summary="Season-Aware Search")
def api_season_aware_search(req: SearchRequest):
    try:
        # Reconstruct internal message objects
        if len(req.msgs) == 0:
            results = search_resources(
                query=req.query, 
                target_uri=req.target_uri, 
                limit=req.limit, 
                score_threshold=req.score_threshold, 
                filter=req.filter
            )
        else:
            internal_msgs = []
            for msg in req.msgs:
                if msg.role == "user":
                    internal_msgs.append(Message.create_user(msg.content))
                else:
                    internal_msgs.append(Message.create_assistant(msg.content))
            results = season_aware_search(
                query=req.query, 
                msgs=internal_msgs, 
                target_uri=req.target_uri, 
                limit=req.limit, 
                score_threshold=req.score_threshold, 
                filter=req.filter
            )
        
        # safely convert to dict
        if hasattr(results, "to_dict"):
            res_data = results.to_dict()
            if hasattr(results, "query_results") and results.query_results:
                res_data["query_results"] = [
                    {
                        "query": results._query_to_dict(qr.query) if hasattr(results, "_query_to_dict") else str(qr.query),
                        "matched_contexts": [results._context_to_dict(c) if hasattr(results, "_context_to_dict") else c.uri for c in qr.matched_contexts],
                        "searched_directories": qr.searched_directories,
                        "thinking_trace": qr.thinking_trace.to_dict() if hasattr(qr.thinking_trace, "to_dict") else {}
                    }
                    for qr in results.query_results
                ]
        else:
            res_data = results
            
        return {"status": "success", "data": res_data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrieval/read_progressively", summary="Read Resources Progressively", response_model=ReadProgressivelyResponse)
def api_read_progressively(req: ReadProgressivelyRequest):
    try:
        result = read_resources_progressively(urls=req.urls)
        return {"status": "success", "data": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/retrieval/read", summary="Read a Specific Resource Level")
def api_read_resource(target: str, level: str = "L2"):
    try:
        data = read_resource(target=target, level=level)
        return {"status": "success", "level": level, "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ==========================================
# Routes: System
# ==========================================
@app.get("/system/status", summary="Get System Status")
def api_get_status():
    try:
        status = get_status()
        return {"status": "success", "data": status}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/system/health", summary="Quick Health Check")
def api_is_healthy():
    try:
        healthy = is_healthy()
        return {"status": "success", "data": healthy}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ==========================================
# Routes: Sessions
# ==========================================
@app.post("/sessions/create", summary="Create Session")
def api_create_session():
    try:
        data = create_session()
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/sessions/list", summary="List Sessions")
def api_list_sessions():
    try:
        data = list_sessions()
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/sessions/{session_id}", summary="Get Session")
def api_get_session(session_id: str):
    if not session_exists(session_id):
        raise HTTPException(status_code=404, detail="Session not found")
    try:
        data = get_session(session_id)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.delete("/sessions/{session_id}", summary="Delete Session")
def api_delete_session(session_id: str):
    try:
        delete_session(session_id)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/sessions/{session_id}/message", summary="Add Message")
def api_add_message(session_id: str, req: AddMessageRequest):
    try:
        data = add_message(session_id, req.role, content=req.content, parts=req.parts)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/sessions/{session_id}/commit", summary="Commit Session")
def api_commit_session(session_id: str):
    try:
        data = commit_session(session_id)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ==========================================
# Routes: Skills
# ==========================================
@app.post("/skills/add", summary="Add Skill")
def api_add_skill(req: Dict[str, Any]):
    try:
        data = add_skill(req)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# ==========================================
# Extended Routes: Resources (Sync / IO)
# ==========================================
@app.post("/resources/export_ovpack", summary="Export OVPack")
def api_export_ovpack(req: ExportOvpackRequest):
    try:
        fd, temp_path = tempfile.mkstemp(suffix=".ovpack")
        os.close(fd)
        
        export_ovpack(req.uri, temp_path)
        
        with open(temp_path, "rb") as f:
            file_bytes = f.read()
            
        try:
            os.remove(temp_path)
        except Exception:
            pass
        
        return Response(
            content=file_bytes, 
            media_type="application/octet-stream"
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/import_ovpack", summary="Import OVPack")
def api_import_ovpack(
    file: UploadFile = File(..., description="Uploaded .ovpack file to import"),
    target: str = Form(..., description="Target URI to unpack to"),
    force: bool = Form(False, description="Force overwrite"),
    vectorize: bool = Form(True, description="Run vectorization on unpacking")
):
    try:
        temp_dir = tempfile.mkdtemp()
        temp_file_path = os.path.join(temp_dir, file.filename)
        
        with open(temp_file_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)
            
        data = import_ovpack(temp_file_path, target, force, vectorize)
        
        try:
            os.remove(temp_file_path)
            os.rmdir(temp_dir)
        except Exception:
            pass
            
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/mkdir", summary="Make Directory")
def api_mkdir(req: MkdirRequest):
    try:
        mkdir(req.uri)
        return {"status": "success"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/resources/stat", summary="Stat Resource")
def api_stat(uri: str):
    try:
        data = stat(uri)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/resources/wait_processed", summary="Wait Operations Processed")
def api_wait_processed(timeout: float = None):
    try:
        data = wait_processed(timeout)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/resources/build_index", summary="Trigger Build Index")
def api_build_index(req: BuildIndexRequest):
    try:
        data = build_index(resource_uris=req.resource_uris)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
