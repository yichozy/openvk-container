# Use an official Python runtime as a parent image from ECR
FROM public.ecr.aws/docker/library/python:3.11-slim-bookworm

WORKDIR /app

# Switch apt mirror to avoid flaky deb.debian.org CDN nodes
RUN sed -i 's/deb.debian.org/cloudfront.debian.net/g' /etc/apt/sources.list.d/debian.sources

# Install supervisor and required tools, then clean up metadata
RUN apt-get update -o Acquire::Retries=3 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends --fix-missing -o Acquire::Retries=3 \
    supervisor \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install uv for fast compilation and dependency resolution
RUN pip install uv

# Configure best pip mirror (Tsinghua) if the global network is slow
RUN curl -s -m 3 https://google.com > /dev/null || \
    uv pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple

COPY requirements.txt .

# Install dependencies using uv and clean up build cache
RUN uv pip install --system --no-cache-dir -r requirements.txt

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

# Create a non-root user for security compliance and change ownership of working directories
RUN groupadd -r appuser && useradd -r -g appuser appuser && \
    chown -R appuser:appuser /app /data

# Switch to the non-root user
USER appuser

# Run supervisor using our custom user-owned config
CMD ["/usr/bin/supervisord", "-c", "/app/supervisord.conf"]
