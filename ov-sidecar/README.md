# grep-sidecar

基于 ripgrep 的文件搜索 HTTP 服务。接收正则表达式和文件过滤条件，返回匹配的文件路径列表。

## 环境变量

| 变量                    | 默认值     | 说明                                                                       |
| ----------------------- | ---------- | -------------------------------------------------------------------------- |
| `GREP_PORT`             | `1935`     | HTTP listening port                                                        |
| `GREP_TIMEOUT`          | `30s`      | Timeout for a single search operation                                      |
| `GREP_MAX_RESULTS`      | `500`      | Maximum number of matched files to return                                  |
| `GREP_MAX_FILESIZE`     | `10M`      | Maximum file size for ripgrep to scan                                      |
| `REDIS_ADDR`            | (Empty)    | Redis address. Empty disables grep result cache                            |
| `REDIS_PASSWORD`        | (Empty)    | Redis password                                                             |
| `REDIS_DB`              | `0`        | Redis DB index                                                             |
| `GREP_CACHE_PREFIX`     | `grep:`    | Redis key prefix                                                           |
| `GREP_CACHE_TTL`        | `120s`     | Grep cache TTL                                                             |
| `OPEN_VIKING_DATA_PATH` | (Required) | Absolute path to the OpenViking data root (e.g., `/data/workspace/viking`) |
| `OPEN_VIKING_ACCOUNT`   | `default`  | Account name used for generating result paths                              |

## Running Locally

```bash
# 1. Install ripgrep
brew install ripgrep

# 进入目录
cd grep-sidecar

# 3. Set the required data path
export OPEN_VIKING_DATA_PATH=/path/to/viking/data

# 4. Start the service
go run .
```

The service will start and listen on `http://localhost:1935`.

## Running with Docker

We provide a convenient shell script to run the sidecar using Docker:

```bash
./run_sider_car.sh
```

Or run it manually:

```bash
# 构建
docker build -t grep-sidecar ./grep-sidecar

docker run -d \
  --name openvk-grep-sidecar \
  -p 1935:1935 \
  -v "$(pwd)/data/workspace:/data/workspace:ro" \
  -e OPEN_VIKING_DATA_PATH=/data/workspace/viking \
  -e OPEN_VIKING_ACCOUNT=default \
  -e GREP_TIMEOUT=30s \
  -e GREP_MAX_RESULTS=500 \
  grep-sidecar
```

## API Documentation

### `POST /grep`

Searches for a regex pattern within a specified directory.

**Request:**

```bash
curl -X POST http://localhost:1935/grep \
  -H 'Content-Type: application/json' \
  -d '{
    "pattern": "(?=.*ITT)(?=.*PD-L1)",
    "directories": ["viking://resources/curation/NSCLC/NCT02453282"],
    "glob": "*.txt",
    "max_results": 5
  }'
```

**Parameters:**

| Field         | Type   | Required | Description                                                                    |
| ------------- | ------ | -------- | ------------------------------------------------------------------------------ |
| `pattern`     | string | Yes      | The regular expression to search for.                                          |
| `directories` | array  | Yes      | The directories to search in. Supports both `viking://...` and relative formats. |
| `glob`        | string | No       | File filter glob pattern (e.g., `*.log`, `**/*.md`).                           |
| `max_results` | int    | No       | Maximum results for this specific request (capped by `GREP_MAX_RESULTS`).      |

_Note: The `directories` entries are automatically resolved using `OPEN_VIKING_DATA_PATH` + `OPEN_VIKING_ACCOUNT`. For example, `viking://resources/curation/cardio` translates to `/data/workspace/viking/default/resources/curation/cardio`._

**Response:**

```json
{
  "status": "success",
  "data": {
    "uris": ["viking://resources/curation/NSCLC/NCT02453282/doc1.txt"],
    "truncated": false
  }
}
```

Matched file paths are returned strictly with the `viking://` scheme.

### `GET /health`

Health check endpoint.

```bash
curl http://localhost:1935/health
# {"status":"ok"}
```

## Advanced Search Patterns

When integrating with the API, you can construct complex search logics using standard regex (as demonstrated in the Go client examples):

- **Logical AND**: Use positive lookaheads. E.g., `(?=.*apple)(?=.*banana)` matches lines containing both "apple" AND "banana" in any order.
- **Logical OR**: Use the pipe operator. E.g., `apple|banana` matches lines containing "apple" OR "banana".
