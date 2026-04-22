# grep-sidecar

基于 ripgrep 的文件搜索 HTTP 服务。接收正则表达式和文件过滤条件，返回匹配的文件路径列表。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `GREP_PORT` | `1935` | HTTP 监听端口 |
| `GREP_DATA_DIR` | `/data` | 搜索根目录 |
| `GREP_TIMEOUT` | `30s` | 单次搜索超时 |
| `GREP_MAX_RESULTS` | `500` | 最大返回文件数 |
| `GREP_MAX_FILESIZE` | `10M` | ripgrep 扫描的最大文件大小 |
| `OPEN_VIKING_DATA_PATH` | `/data/workspace/viking` | 结果路径转换前缀 |
| `OPEN_VIKING_ACCOUNT` | `default` | 结果路径转换账号名 |
| `ENV` | `dev` | 设为 `prod` 跳过 .env 加载 |

## 本地启动

```bash
# 前置：安装 ripgrep
brew install ripgrep

# 进入目录
cd grep-sidecar

# 复制并编辑环境变量
cp .env.example .env
# 按需修改 GREP_DATA_DIR 等配置

# 启动
go run .
```

服务启动后监听 `http://localhost:1935`。

## Docker 启动

```bash
# 构建
docker build -t grep-sidecar ./grep-sidecar

# 运行
docker run -d \
  --name grep-service \
  -p 1935:1935 \
  -v /path/to/your/data:/data:ro \
  -e GREP_DATA_DIR=/data \
  -e OPEN_VIKING_DATA_PATH=/data/workspace/viking \
  -e OPEN_VIKING_ACCOUNT=default \
  -e GREP_TIMEOUT=30s \
  -e GREP_MAX_RESULTS=500 \
  grep-sidecar
```

或通过 docker-compose：

```bash
docker compose up grep-service -d
```

## API

### POST /grep

```bash
curl -X POST http://localhost:1935/grep \
  -H 'Content-Type: application/json' \
  -d '{
    "pattern": "indicationName",
    "directory": "viking://resources/curation/cardio",
    "glob": "*.txt"
  }'
```

**参数：**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `pattern` | string | 是 | 正则表达式 |
| `directory` | string | 是 | 搜索目录，支持 `viking://resources/curation/cardio` 或 `resources/curation/cardio` 两种格式 |
| `glob` | string | 否 | 文件过滤，如 `*.log`、`**/*.py` |
| `max_results` | int | 否 | 本次请求最大返回数，不超过全局上限 |

`directory` 字段会自动拼接 `OPEN_VIKING_DATA_PATH` + `OPEN_VIKING_ACCOUNT` 生成完整路径。例如 `viking://resources/curation/cardio` 会被解析为 `/data/workspace/viking/default/resources/curation/cardio`。两种格式等效：

```
"directory": "viking://resources/curation/cardio"
"directory": "resources/curation/cardio"
```

**响应：**

```json
{
  "status": "success",
  "data": {
    "files": ["viking://resources/curation/cardio/doc1.txt"],
    "truncated": false
  }
}
```

匹配的文件路径同样以 `viking://` 前缀返回。

### 示例

**搜索目录下包含关键词的文件：**

```bash
curl -s -X POST http://localhost:1935/grep \
  -H 'Content-Type: application/json' \
  -d '{
    "pattern": "heart failure",
    "directory": "viking://resources/curation/cardio"
  }' | jq
```

```json
{
  "status": "success",
  "data": {
    "files": [
      "viking://resources/curation/cardio/guideline_v2.txt",
      "viking://resources/curation/cardio/trials/esc_2024.txt"
    ],
    "truncated": false
  }
}
```

**限定文件类型：**

```bash
curl -s -X POST http://localhost:1935/grep \
  -H 'Content-Type: application/json' \
  -d '{
    "pattern": "GLP-1",
    "directory": "viking://resources",
    "glob": "*.md"
  }' | jq
```

```json
{
  "status": "success",
  "data": {
    "files": ["viking://resources/drug_notes/glp1_review.md"],
    "truncated": false
  }
}
```

**使用正则表达式：**

```bash
curl -s -X POST http://localhost:1935/grep \
  -H 'Content-Type: application/json' \
  -d '{
    "pattern": "error|fatal|panic",
    "directory": "viking://resources",
    "glob": "*.log"
  }' | jq
```

```json
{
  "status": "success",
  "data": {
    "files": [
      "viking://resources/logs/app.log",
      "viking://resources/logs/worker.log"
    ],
    "truncated": false
  }
}
```

### GET /health

```bash
curl http://localhost:1935/health
# {"status":"ok"}
```
