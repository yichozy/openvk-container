#!/bin/bash
OPENVIKING_CONFIG_FILE=./.openviking/ov.conf python -m uvicorn main:app --host 0.0.0.0 --port 1934
