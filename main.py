from contextlib import asynccontextmanager
from typing import List, Union, Dict, Any, Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from openviking.message import Message
from service.resources import (
    add_resource,
    list_resources,
    get_resource_relations,
    move_resource,
    delete_resource,
    link_resources,
    unlink_resources
)
from service.retrieval import (
    find_resources,
    season_aware_search,
    read_resources_progressively,
    read_resource
)

@asynccontextmanager
async def lifespan(app: FastAPI):
    yield
    from service.client import OpenVK
    OpenVK.close_client()

app = FastAPI(title="OpenViking Client API", lifespan=lifespan)

# --- Pydantic Models for Resources ---

class AddResourceRequest(BaseModel):
    path_or_url: str = Field(..., description="Path or URL to add")
    target: str = Field(..., description="Target URI (e.g., viking://...)")
    reason: str = Field("", description="Reason for adding the resource")

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

class MessageItem(BaseModel):
    role: str = Field(..., description="Role of the sender ('user' or 'assistant')")
    content: str = Field(..., description="Message content")

class SearchRequest(BaseModel):
    query: str = Field(..., description="Search query string")
    msgs: List[MessageItem] = Field(..., description="List of previous conversation messages")
    target_uri: str = Field("", description="Optional target URI context")

class MatchedContextInput(BaseModel):
    uri: str
    abstract: str = ""
    is_leaf: bool = True

class ReadProgressivelyRequest(BaseModel):
    resources: List[MatchedContextInput] = Field(..., description="List of resource contexts to read context from")


# ==========================================
# Routes: Resources
# ==========================================

@app.post("/resources/add", summary="Add a Resource")
def api_add_resource(req: AddResourceRequest):
    try:
        status = add_resource(path_or_url=req.path_or_url, target=req.target, reason=req.reason)
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


# ==========================================
# Routes: Retrieval
# ==========================================

@app.post("/retrieval/find", summary="Find Resources")
def api_find_resources(req: FindRequest):
    try:
        results = find_resources(query=req.query, target_uri=req.target_uri)
        return {"status": "success", "data": results}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrieval/search", summary="Season-Aware Search")
def api_season_aware_search(req: SearchRequest):
    try:
        # Reconstruct internal message objects
        internal_msgs = []
        for msg in req.msgs:
            if msg.role == "user":
                internal_msgs.append(Message.create_user(msg.content))
            else:
                internal_msgs.append(Message.create_assistant(msg.content))
        
        results = season_aware_search(query=req.query, msgs=internal_msgs, target_uri=req.target_uri)
        return {"status": "success", "data": results}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrieval/read_progressively", summary="Read Resources Progressively")
def api_read_progressively(req: ReadProgressivelyRequest):
    try:
        # Convert internal mocked/struct representation
        # It needs objects behaving like 'MatchedContext' structurally. Pydantic input passes objects that mimic it via duck typing if mapped.
        class TempContext:
            def __init__(self, uri, abstract, is_leaf):
                self.uri = uri
                self.abstract = abstract
                self.is_leaf = is_leaf

        contexts = [TempContext(r.uri, r.abstract, r.is_leaf) for r in req.resources]
        result = read_resources_progressively(contexts)
        return {"status": "success", "data": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/retrieval/read", summary="Read a Specific Resource Level")
def api_read_resource(target: str, level: str = "L2"):
    try:
        data = read_resource(target=target, level=level)
        return {"status": "success", "data": data}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
