# Use an official Python runtime as a parent image from ECR
FROM public.ecr.aws/docker/library/python:3.11-slim

# Set the working directory in the container
WORKDIR /app

# Install necessary build tools, curl, and certificates
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    build-essential \
    supervisor \
    cmake \
    libc6 \
    golang \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Auto-detect and configure best pip mirror (use Tsinghua if global network is unreachable)
RUN curl -s -m 3 https://google.com > /dev/null || \
    pip config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple


# Copy requirements.txt and install dependencies
COPY requirements.txt .
RUN pip install -r requirements.txt

# Create directory for agent workspaces and logs
RUN mkdir -p /data/workspace /data/log

# Expose the default OpenViking port
EXPOSE 1933
EXPOSE 1934

# Copy the supervisor configuration file
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Copy source code
COPY main.py .
COPY service ./service

CMD ["/usr/bin/supervisord", "-n"]
