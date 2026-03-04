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

4. **Verify the Server**

   Check if the server is healthy:

   ```bash
   curl http://localhost:1933/health
   ```

   You should see `{"status": "ok"}`.

## Connecting from Python SDK

```python
import openviking as ov

client = ov.SyncHTTPClient(url="http://localhost:1933")
client.initialize()
print("Connected to OpenViking Container!")
```
