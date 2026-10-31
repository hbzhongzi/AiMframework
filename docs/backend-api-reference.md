# Gemma 4 12B 后端复用接口说明

## 1. 服务地址

推荐所有外部调用统一访问 Go 网关：

```text
http://127.0.0.1:8081
```

网关负责 API 反代、SSE 流式转发、跨域和可选鉴权。`8080` 是内部 llama-server 端口，除非做本机调试，否则不要让业务代码直接依赖它。

| 地址 | 用途 | 建议 |
|---|---|---|
| `http://127.0.0.1:8081` | 可复用统一入口 | 推荐 |
| `http://127.0.0.1:8080` | llama-server 内部接口 | 仅调试 |

默认监听本机回环地址，不能被局域网其它机器访问。若要跨机器调用，需要显式配置监听地址、防火墙和鉴权。

## 2. 前置启动

确保以下服务已经启动：

```powershell
Test-Path D:\llama-cpp\llama-server.exe
Test-Path D:\models\gemma-4-12b-Q4_K_M.gguf
```

推荐直接启动桌面端：

```powershell
.\dist\gemma-desktop.exe
```

或使用脚本：

```powershell
.\scripts\start-all.bat
```

等待模型状态变为 `running` 后再调用聊天接口。

## 3. 健康检查

### `GET /health`

检查 Go 网关和上游模型服务。

```powershell
curl.exe http://127.0.0.1:8081/health
```

就绪响应：

```json
{"status":"ok","upstream":"http://127.0.0.1:8080"}
```

模型加载中或上游未启动时返回 HTTP `503`：

```json
{"status":"degraded","upstream":"http://127.0.0.1:8080"}
```

调用方应在收到 `503` 时等待后重试，不要立即判定服务永久失败。

## 4. 模型列表

### `GET /v1/models`

透传 llama-server 的 OpenAI 兼容模型列表。

```powershell
curl.exe http://127.0.0.1:8081/v1/models
```

不同 llama.cpp 版本返回的模型 ID 可能不同。应用侧建议使用：

```text
gemma-4
```

聊天请求中的 `model` 字段保留 OpenAI 兼容格式；单模型部署时实际推理模型由服务启动参数决定。

## 5. 聊天接口

### `POST /v1/chat/completions`

请求头：

```http
Content-Type: application/json
Authorization: Bearer not-needed
```

最小非流式请求：

```json
{
  "model": "gemma-4",
  "messages": [
    {"role": "user", "content": "用一句话解释什么是量化"}
  ],
  "stream": false,
  "temperature": 0.7,
  "top_p": 0.95,
  "max_tokens": 1024
}
```

PowerShell：

```powershell
$body = @{
  model = "gemma-4"
  messages = @(
    @{ role = "user"; content = "你好，请介绍一下你自己" }
  )
  stream = $false
  temperature = 0.7
  top_p = 0.95
  max_tokens = 1024
} | ConvertTo-Json -Depth 6

Invoke-RestMethod `
  http://127.0.0.1:8081/v1/chat/completions `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

### 流式请求

将 `stream` 改为 `true`：

```json
{
  "model": "gemma-4",
  "messages": [
    {"role": "user", "content": "写一首四句短诗"}
  ],
  "stream": true,
  "temperature": 0.8,
  "top_p": 0.95,
  "top_k": 64,
  "max_tokens": 1024
}
```

响应类型：

```text
text/event-stream
```

每个数据块格式为：

```text
data: {"choices":[{"delta":{"content":"文本片段"}}]}

```

结束标记：

```text
data: [DONE]
```

调用方必须持续读取 SSE，不能只等待一次完整 JSON 响应。

## 6. Python 调用

安装 SDK：

```powershell
python -m pip install openai
```

非流式：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8081/v1",
    api_key="not-needed",
)

response = client.chat.completions.create(
    model="gemma-4",
    messages=[
        {"role": "user", "content": "解释一下 GGUF 是什么"},
    ],
    stream=False,
    temperature=0.7,
    max_tokens=1024,
)

print(response.choices[0].message.content)
```

流式：

```python
stream = client.chat.completions.create(
    model="gemma-4",
    messages=[{"role": "user", "content": "列出三个本地部署优点"}],
    stream=True,
)

for chunk in stream:
    text = chunk.choices[0].delta.content or ""
    print(text, end="", flush=True)
```

## 7. JavaScript 调用

```javascript
const response = await fetch("http://127.0.0.1:8081/v1/chat/completions", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": "Bearer not-needed"
  },
  body: JSON.stringify({
    model: "gemma-4",
    messages: [{ role: "user", content: "你好" }],
    stream: false,
    temperature: 0.7,
    max_tokens: 1024
  })
});

const data = await response.json();
console.log(data.choices?.[0]?.message?.content);
```

## 8. 模型控制接口

这些接口用于本机后台控制，不建议暴露给普通业务客户端。

### `GET /admin/status`

返回当前进程、健康状态和配置：

```json
{
  "state": "running",
  "process": true,
  "upstream": true,
  "llama_bin": "D:\\llama-cpp\\llama-server.exe",
  "model_path": "D:\\models\\gemma-4-12b-Q4_K_M.gguf",
  "ctx": "16384",
  "ngl": "999",
  "upstream_url": "http://127.0.0.1:8080"
}
```

状态值：

| 状态 | 含义 |
|---|---|
| `stopped` | 没有由后台管理的模型进程 |
| `starting` | 进程已启动，模型尚未健康 |
| `running` | 模型已健康，可接受聊天请求 |

### `POST /admin/start`

启动一个 GGUF 模型。模型必须位于配置的模型目录，默认是 `D:\models`。

```json
{
  "model_path": "D:\\models\\gemma-4-12b-Q4_K_M.gguf",
  "ctx": "16384",
  "ngl": "999"
}
```

### `POST /admin/stop`

停止当前由后台管理的 llama-server：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/stop -Method Post
```

### `POST /admin/restart`

按新参数停止旧模型并重新启动：

```powershell
$body = @{ model_path = "D:\models\gemma-4-12b-Q4_K_M.gguf"; ctx = "8192"; ngl = "999" } | ConvertTo-Json
Invoke-RestMethod http://127.0.0.1:8081/admin/restart -Method Post -ContentType "application/json" -Body $body
```

### `GET /admin/log`

返回最近的 llama-server 日志行，后台最多保留 2000 行：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/log
```

### `GET /admin/gpu`

通过 `nvidia-smi` 返回 GPU 名称、显存占用、总显存和利用率：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/gpu
```

## 9. 鉴权与配置

默认本机开发配置不启用网关鉴权。部署到其它机器前设置：

```powershell
$env:API_KEY = "替换为强随机密钥"
$env:PORT = "8081"
$env:UPSTREAM = "http://127.0.0.1:8080"
$env:MODEL_DIR = "D:\models"
```

启用 `API_KEY` 后，业务调用必须发送：

```http
Authorization: Bearer <API_KEY>
```

模型控制接口和聊天接口会同时受到该鉴权中间件保护。`/admin` 具备启动本机进程能力，只应绑定 `127.0.0.1` 或放在受保护的内网中。

## 10. 常见错误

| HTTP 状态 | 原因 | 处理 |
|---|---|---|
| `401` | `API_KEY` 已启用但请求头缺失或错误 | 添加 Bearer Token |
| `404` | URL 或方法错误 | 确认使用 `/v1/chat/completions` 和 POST |
| `502` | Go 网关无法连接 llama-server | 检查模型状态和 8080 |
| `503` | 模型仍在加载或未启动 | 轮询 `/health` 或 `/admin/status` |
| `400` | JSON、模型路径或参数不合法 | 检查请求体 |

## 11. 推荐复用方式

业务系统只依赖以下三个入口即可：

```text
GET  http://127.0.0.1:8081/health
GET  http://127.0.0.1:8081/v1/models
POST http://127.0.0.1:8081/v1/chat/completions
```

模型启停属于运维控制面，单独由桌面后台或受保护的管理服务调用 `/admin/*`，不要让普通聊天客户端直接调用模型控制接口。
