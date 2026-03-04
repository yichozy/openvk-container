# Use an official Python runtime as a parent image from ECR
FROM public.ecr.aws/docker/library/python:3.11-slim

# Set the working directory in the container
WORKDIR /app

# Install uv for faster python package installation and necessary build tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    supervisor \
    cmake \
    libc6 \
    golang \
    && rm -rf /var/lib/apt/lists/*
RUN pip install uv

# Copy requirements.txt and install dependencies
COPY requirements.txt .
RUN uv pip install --system -r requirements.txt

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
