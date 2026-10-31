# Gemma Go 后端（gemma-backend）

Go（chi/v5）实现的轻量网关，作为 Gemma 4 12B 本地部署的**前端统一入口**：

- 反向代理 llama-server 的 OpenAI 兼容 API：`/v1/chat/completions`（SSE 流式逐块透传）、`/v1/models`。
- 托管三个前端 HTML：`/chat`（gemma-chat.html）、`/launcher`（open-webui-example.html）、`/admin`（admin.html）。
- `/admin/status|start|stop|restart|log|gpu`：统一控制模型进程、参数、日志和显存状态。

接口复用说明见：`../docs/backend-api-reference.md`。
- `/health` 同时探测上游 llama-server；可选 API Key 鉴权、CORS。

## 目录
- `main.go` — 路由、中间件、静态托管、启动入口
- `proxy.go` — SSE 透传反向代理
- `config.go` — 环境变量配置
- `go.mod` — 依赖（github.com/go-chi/chi/v5）

## 运行
```powershell
cd backend
# 环境变量（均有默认值，可省略）
#   PORT=8081
#   UPSTREAM=http://127.0.0.1:8080
#   UPSTREAM_API_KEY=not-needed
#   MODEL=gemma-4
#   STATIC_DIR=../frontend    # 指向前端目录（含 HTML）
#   API_KEY=                # 留空=不鉴权；设值则客户端须带 Authorization: Bearer <API_KEY>

go run .                    # 开发模式
# 或直接使用已编译产物
gemma-backend.exe
```

## 入口
- 前端：`http://localhost:8081/chat`、`http://localhost:8081/launcher`
- API：`http://localhost:8081/v1/chat/completions`
- 健康检查：`http://localhost:8081/health`

## 说明
- `gemma-chat.html` 经本服务（http）访问时，API Base 自动设为同源 `/v1`；直接双击 `file://` 打开时回退直连 llama-server `:8080`。
- 方案 A（Open WebUI）可直接连 llama-server `:8080`，也可把连接 URL 改为 `http://host.docker.internal:8081/v1` 走本服务。
- 推理仍由 llama.cpp / llama-server 在 GPU 上完成，本服务只做协议网关与静态托管。
