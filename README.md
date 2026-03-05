# OpenViking Container

This repository contains the necessary files to containerize and run the **OpenViking** ecosystem.
OpenViking is an open-source context database designed specifically for AI Agents.

This container runs two primary services supervised together:

1. **OpenViking Server (Port 1933):** The core OpenViking backend (AGFS, Vector Index, Queue Manager, etc.).
2. **OpenViking Client API (Port 1934):** A FastAPI-based REST API that wraps the synchronous OpenViking client SDK, providing easy HTTP access to the database's capabilities.

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

   Edit the `ov.conf` file to include your LLM/Embedding API Keys and endpoints. The workspace will be automatically stored in the `./data` directory relative to this `docker-compose.yml`.

   _Example: Replacing `YOUR_VLM_API_KEY` for OpenAI and `YOUR_EMBEDDING_API_KEY` for Volcengine (or you can use OpenAI for both)._

3. **Start the Services**

   Run Docker Compose in detached mode:

   ```bash
   docker compose up -d
   ```

4. **Verify the Services**

   Check if the OpenViking core server is healthy:

   ```bash
   curl http://localhost:1933/health
   ```

   _Expected:_ `{"status": "ok"}`

   Verify that the Client REST API is running by accessing the Interactive Docs (Swagger UI) in your browser:  
   [http://localhost:1934/docs](http://localhost:1934/docs)

## Client REST API (Port 1934)

The container includes a built-in Client REST API powered by FastAPI. This API acts as an HTTP wrapper around the native `openviking` client, allowing you to seamlessly manage resources and perform intelligent retrievals from any language via standard HTTP requests.

### API Capabilities

- **Resources (`/resources/*`):** Add (via URL or direct file upload), list, move, link, delete, and perform file system operations like `mkdir`, `stat`, `tree`, `grep`, and `glob`. Also supports importing and exporting `.ovpack` archives.
- **Retrieval (`/retrieval/*`):** Perform vector-based season-aware semantic searches, specific text finds, and progressive reading.
- **Sessions (`/sessions/*`):** Chat session management, including creation, listing, message addition, and memory commits.
- **Skills (`/skills/*`):** Register new tools and AI skills dynamically.
- **System (`/system/*`):** Check container health status and internal component metrics.

### File Uploads Example

To upload a document to your AI's context via the client API:

```bash
curl -X 'POST' \
  'http://localhost:1934/resources/add_file' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'file=@/path/to/your/document.pdf' \
  -F 'target=viking://resources/docs/' \
  -F 'reason=Adding new project specification'
```

## Connecting from Python SDK directly to Server (Port 1933)

If you are using Python, you can connect directly to the underlying OpenViking Server instead of using the Client API:

```python
import openviking as ov

client = ov.SyncHTTPClient(url="http://localhost:1933")
client.initialize()
print("Connected to OpenViking Container!")
```
