# OpenViking Server Container

This repository contains the necessary files to containerize and run the `openviking-server`.
OpenViking is an open-source context database designed specifically for AI Agents.

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

3. **Start the Server**

   Run Docker Compose in detached mode:

   ```bash
   docker compose up -d
   ```

4. **Verify the Services**

   Check if the OpenViking core server is healthy:

   ```bash
   curl http://localhost:1933/health
   ```

   You should see `{"status": "ok"}`.

   Verify that the Client REST API is running by accessing the Swagger UI in your browser:  
   [http://localhost:1934/docs](http://localhost:1934/docs)

## Client REST API

The container includes a built-in Client REST API powered by FastAPI on port `1934` (mapped from container port `1934`). This API acts as an HTTP wrapper around the native `openviking` client, allowing you to seamlessly manage resources and perform intelligent retrievals.

- **Interactive API Docs (Swagger UI):** [http://localhost:1934/docs](http://localhost:1934/docs)

### API Capabilities

- **Resources (`/resources/*`):** Add, list, move, link, delete, and perform file system operations like `mkdir`, `stat`, `tree`, `grep`, and `glob`. Also supports importing and exporting `.ovpack` archives.
- **Retrieval (`/retrieval/*`):** Perform vector-based season-aware semantic searches, specific text finds, and progressive reading.
- **Sessions (`/sessions/*`):** Chat session management, including creation, listing, message addition, and memory commits.
- **Skills (`/skills/*`):** Register new tools and AI skills dynamically.
- **System (`/system/*`):** Check container health status and internal component metrics.

## Connecting from Python SDK

```python
import openviking as ov

client = ov.SyncHTTPClient(url="http://localhost:1933")
client.initialize()
print("Connected to OpenViking Container!")
```
