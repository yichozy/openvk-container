# OpenViking Container

This repository contains the necessary files to containerize and run the **OpenViking** ecosystem.
OpenViking is an open-source context database designed specifically for AI Agents.

This container runs a standalone FastAPI REST API (Port 1934) that wraps the `openviking` client SDK. It initializes its own local backend instance internally and allows you to seamlessly manage resources and perform intelligent retrievals from any programming language via standard HTTP requests.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed.
- [Docker Compose](https://docs.docker.com/compose/install/) installed.

## Setup Instructions

1. **Prepare the configuration file**

   Copy the example configuration file:

   ```bash
   cp ov.conf.example ov.conf
   ```

2. **Configure your API keys**

   Edit the `ov.conf` file to include your LLM/Embedding API Keys and endpoints.

   _Example: Replacing `YOUR_VLM_API_KEY` for OpenAI and `YOUR_EMBEDDING_API_KEY` for Volcengine (or you can use OpenAI for both)._

3. **Data Persistence**

   The Docker container automatically persists OpenViking's internal storage, vector indexes, and contextual data by mounting the local `./data` directory into the container's `/app/data` pathway. You don't need to do anything extra; all your states will survive container restarts!

4. **Start the API**

   Run Docker Compose in detached mode:

   ```bash
   docker compose up -d
   ```

5. **Verify the API**

   Verify that the Client REST API is running by accessing the Interactive Docs (Swagger UI) in your browser:  
   [http://localhost:1934/docs](http://localhost:1934/docs)

## Local Development (VS Code)

A `.vscode/launch.json` configuration is included to easily debug the FastAPI interface natively:

1. Make sure your Python environment has dependencies installed (`pip install -r requirements.txt`).
2. Press **F5** in VS Code with the `"Python: FastAPI (uvicorn)"` profile.
3. It will automatically load `.openviking/ov.conf` into your variables and execute `main.py` with automatic code reloading.

## Client REST API (Port 1934)

The container includes a built-in Client REST API powered by FastAPI. This API acts as an HTTP wrapper around the native `openviking` client, allowing you to seamlessly manage resources and perform intelligent retrievals from any language via standard HTTP requests.

### API Capabilities

- **Resources (`/resources/*`):** Add (via URL, direct file upload, or raw byte stream), replace (via URL or file), list, move, link, unlink, delete, and perform file system operations like `mkdir`, `stat`, `tree`, `grep`, and `glob`. Also supports importing and exporting `.ovpack` archives, waiting for async operations (`wait_processed`), and manual index building (`build_index`).
- **Retrieval (`/retrieval/*`):** Perform vector-based season-aware semantic searches, specific text finds, and progressive reading (with explicit `level` properties returned in single reads, and clear content prefixes like "Abstract: ", "Overview: " and "File content: " in progressive arrays).
- **Sessions (`/sessions/*`):** Chat session management, including creation, listing, retrieval, deletion, message addition, and memory commits.
- **Skills (`/skills/*`):** Register new tools and AI skills dynamically.
- **System (`/system/*`):** Check container health status and internal component metrics.

### Multi-Tenancy Support

The API now supports **multi-tenancy**, allowing you to isolate data and operations for different tenants, workspaces, or users. Each tenant maintains its own separate data directory and client instance.

**How it works:**

- Every API endpoint accepts an optional `tenant_id` parameter (default: `"workspace"`)
- Each tenant's data is stored in a separate directory: `./data/{tenant_id}/`
- Tenant client instances are cached and reused for efficiency
- You can manage completely isolated knowledge bases within a single container

**Usage Examples:**

```bash
# Using query parameters (GET requests)
curl -G 'http://localhost:1934/resources/list' \
  --data-urlencode 'target=viking://' \
  --data-urlencode 'tenant_id=myapp'

# Using form data (POST requests with file uploads)
curl -X 'POST' \
  'http://localhost:1934/resources/add_file' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'file=@document.pdf' \
  -F 'parent=viking://resources/' \
  -F 'tenant_id=user123'

# Using JSON body (POST requests)
curl -X 'POST' \
  'http://localhost:1934/retrieval/search?tenant_id=analytics' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "data pipeline architecture",
    "limit": 5
  }'
```

**Common Use Cases:**

- **SaaS Applications:** Isolate data per customer/organization
- **Multi-User Systems:** Separate knowledge bases for different users
- **Development Environments:** Use different tenants for dev/staging/prod
- **A/B Testing:** Maintain separate datasets for experimentation

**Note:** If no `tenant_id` is specified, the API defaults to using `"workspace"`.

### Retrieval Example

**Advanced Metadata Filtering (`/retrieval/find` and `/retrieval/search`)**

The search endpoints support advanced metadata filtering natively by exposing the `filter` field. You can pass a robust JSON Abstract Syntax Tree (AST) directly to filter results before the vector search executes. 

For example, to search for resources where the `level` is 2 **AND** the `uri` contains either `docs` **OR** `tutorials`:

```bash
curl -X 'POST' \
  'http://localhost:1934/retrieval/search' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "query": "deployment details",
  "limit": 5,
  "filter": {
    "op": "and",
    "conds": [
      {
        "op": "==",
        "field": "level",
        "value": 2
      },
      {
        "op": "or",
        "conds": [
          {"op": "contains", "field": "uri", "substring": "docs"},
          {"op": "contains", "field": "uri", "substring": "tutorials"}
        ]
      }
    ]
  }
}'
```

**Reading a Specific Resource Level (`/retrieval/read`)**

The `read` endpoint now includes the explicitly requested `level` in its successful JSON response. This allows calling clients to definitively know what level (L0: Abstract, L1: Overview, L2: Full Content) was retrieved:

```bash
curl -G 'http://localhost:1934/retrieval/read' \
  --data-urlencode 'target=viking://resources/docs/project.md' \
  --data-urlencode 'level=L2'
```

**Response:**

```json
{
  "status": "success",
  "level": "L2",
  "data": "# Project Title\n..."
}
```

**Recursive Navigation Search (`/retrieval/recursive_search`)**

The `recursive_search` endpoint performs a contextual search capable of propagating scores recursively through directories. It takes parameters such as `topK` (number of top results) and `max_relations`.

```bash
curl -X 'POST' \
  'http://localhost:1934/retrieval/recursive_search' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "query": "system architecture",
  "topK": 5,
  "max_rounds": 3,
  "max_relations": 3,
  "context_type": "RESOURCE"
}'
```

### File Uploads Example

There are two primary ways to upload a document to your AI's context via the client API. Optional parameters such as `replace`, `wait`, `build_index`, and `instruction` can also be provided.

**1. Using Multipart Form Uploads (`add_file`)**

You can specify either `parent` (the destination directory) OR `to` (the exact destination URI including the filename).

Using `parent` (filename will be automatically derived from the uploaded file):
```bash
curl -X 'POST' \
  'http://localhost:1934/resources/add_file' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'file=@/path/to/your/document.pdf' \
  -F 'parent=viking://resources/docs/' \
  -F 'reason=Adding new project specification' \
  -F 'replace=true' \
  -F 'wait=true' \
  -F 'tenant_id=myapp'
```

Using `to` (specifying exact filename):
```bash
curl -X 'POST' \
  'http://localhost:1934/resources/add_file' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'file=@/path/to/your/document.pdf' \
  -F 'to=viking://resources/docs/my_spec.pdf' \
  -F 'reason=Adding new project specification' \
  -F 'tenant_id=myapp'
```

**2. Using Raw Byte Streams (`add_bytes`)**

This bypasses `multipart/form-data` entirely and consumes the raw request body payload, which is optimal for integrations avoiding heavy multipart wrapping routines:

Using `parent` (filename provided via query parameters):
```bash
curl -X 'POST' \
  'http://localhost:1934/resources/add_bytes?filename=document.pdf&parent=viking://resources/docs/&reason=Uploading%20spec&tenant_id=myapp' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/octet-stream' \
  --data-binary '@/path/to/your/document.pdf'
```

Alternatively, you can provide the exact target URI with `to`: `...?filename=document.pdf&to=viking://resources/docs/document.pdf`.
