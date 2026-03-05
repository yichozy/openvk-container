FROM public.ecr.aws/docker/library/python:3.13-slim

WORKDIR /app


COPY requirements.txt .

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends golang git build-essential cmake supervisor

# Install python dependencies
RUN pip install -r requirements.txt

# Remove build dependencies
RUN apt-get purge -y --auto-remove golang git build-essential cmake && \
    rm -rf /var/lib/apt/lists/*

# Create directory for agent workspaces and logs
RUN mkdir -p /data/workspace /data/log

# Expose the default OpenViking and Web API ports
EXPOSE 1933
EXPOSE 1934

# Copy the standalone supervisor configuration file
COPY supervisord.conf /app/supervisord.conf

# Copy source code
COPY main.py .
COPY service ./service

# Run supervisor using our custom user-owned config
CMD ["/usr/bin/supervisord", "-c", "/app/supervisord.conf"]
# CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "1934"]

# openviking-server --config /app/ov.conf --host 0.0.0.0 --port 1933
# CMD ["openviking-server", "--config", "/app/ov.conf", "--host", "0.0.0.0", "--port", "1933"]


