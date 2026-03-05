FROM python:3.13-slim

WORKDIR /app

ENV OPENVIKING_CONFIG_FILE=/app/ov.conf

COPY requirements.txt .

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends golang git build-essential cmake

# Install python dependencies
RUN pip install -r requirements.txt

# Remove build dependencies
RUN apt-get purge -y --auto-remove golang git build-essential cmake && \
    rm -rf /var/lib/apt/lists/*

# Create directory for agent workspaces and logs
RUN mkdir -p /data/workspace /data/log

# Expose the Web API port
EXPOSE 1934

# Copy source code
COPY main.py .
COPY service ./service

# Start the FastAPI runtime directly
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "1934"]
