# Gemma 4 12B 本地部署 · 综合开发落地文档

> 整合范围：**后端（llama.cpp / llama-server）** + **图形化前端（Open WebUI 方案 A + 零依赖 HTML 方案 B）** + 验证 / 排错 / 运维。
> 目标硬件：NVIDIA GeForce RTX 4080 16GB｜模型格式：GGUF Q4_K_M｜后端：llama.cpp（llama-server）｜场景：纯文本对话 / 推理，不加载多模态投影器。
> 前身文档：`deploy-guide.md`（后端详细参数参考），本文件为其综合版，前端部分以本文件为准。

> **当前目录说明（2026-10-31）**：项目已按 `backend/`、`desktop/`、`frontend/`、`scripts/`、`docs/`、`dist/` 分类整理。桌面控制后台的当前落地方案和验收标准以 `docs/windows-desktop-admin-development.md` 为准。
> 后端复用接口、Python/JavaScript 调用示例见 `docs/backend-api-reference.md`。

---

## 0. 交付物清单

| 文件 | 类型 | 作用 |
|------|------|------|
| `Gemma4-12B-综合部署落地文档.md` | 文档 | 本综合落地文档（单一可信源） |
| `deploy-guide.md` | 文档 | 后端详细参数参考（可单独查阅） |
| `start-gemma-server.bat` | 脚本 | 一键启动 llama-server（32K + 全 GPU 卸载 + jinja） |
| `gemma-chat.html` | 前端（方案 B） | 零依赖、流式输出的本地聊天页，连 `:8080` |
| `open-webui-example.html` | 前端（方案 A） | Open WebUI 示例 + 启动台，浅/深色双主题，三标签 |
| `backend/` | 后端（Go） | Go 服务（chi/v5）：反代 llama-server 的 OpenAI API（含 SSE）+ 托管 HTML 前端 |

---

## 1. 目标与架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                           Windows 主机 (RTX 4080 16GB)                 │
│                                                                        │
│   图形化前端                  Go 后端 (chi, :8081)        llama-server   │
│   ┌────────────────┐  HTTP    ┌───────────────────┐  OpenAI   ┌──────────┐ │
│   │ 方案A Open     │ ───────▶ │ 反代 /v1 (SSE)    │ ─────────▶│ :8080    │ │
│   │ WebUI (:3000) │          │ 托管 HTML 前端    │  API /v1  │ llama.cpp│ │
│   │ 方案B html    │          └───────────────────┘           │ CUDA 推理│ │
│   │ (经 Go 托管)  │                                              └────┬─────┘ │
│   └────────────────┘                                                    │    │
│                                                          D:\models\    │    │
│                                                          gemma-4-...   │    │
└──────────────────────────────────────────────────────────────────────┘
```

- **后端**：`llama-server.exe` 监听 `127.0.0.1:8080`，暴露 OpenAI 兼容 API（`/v1/chat/completions`、`/v1/models`、`/health`）。
- **前端**：方案 A（Open WebUI）功能最全（多会话 / 历史 / RAG / 用户管理），已选定；方案 B（`gemma-chat.html`）零安装、双击即用，适合快速验证。

---

## 2. 部署前确认（逐项核对）

### 2.1 GPU 与驱动
```powershell
nvidia-smi
```
确认：能看到 `RTX 4080`，右上角 `CUDA Version` 正常（≥ 12.4 即可，你的驱动 `32.0.16.1088` 已足够新）。
> 此处的 CUDA 版本是**驱动支持的最高版本**，不等于需装 CUDA Toolkit；llama.cpp 的 CUDA 预编译包自带运行库。

### 2.2 磁盘空间
- Q4_K_M 权重约 **6.9 – 7.4 GB**；预留 **10 GB+** 空闲（含临时文件）。
- 建议路径简短、无空格、无中文：`D:\models\`、`D:\llama-cpp\`。

### 2.3 系统内存
- **16 GB** 可运行；**32 GB** 体验更从容（非硬性门槛）。

### 2.4 网络与运行环境
- 能访问 `github.com`（下载 llama.cpp / 权重）。
- 选方案 A：需已安装 **Docker Desktop（Windows，开启 WSL2）**。
- 选方案 B：仅需任意现代浏览器（Chrome / Edge / Firefox），无需联网外网。

---

## 3. 后端部署（llama.cpp / llama-server）

### 3.1 下载 llama.cpp CUDA 预编译（两个包，免编译）
打开 Releases：`https://github.com/ggml-org/llama.cpp/releases`，取 **Latest**（标签形如 `bXXXX`）下的两个 Asset：

| 文件 | 命名特征 | 用途 |
|------|----------|------|
| 主程序包 | `llama-<tag>-bin-win-cuda-12.4-x64.zip` | 含 `llama-server.exe`、`llama-cli.exe` |
| CUDA 运行库 | `cudart-lama-bin-win-cuda-12.4-x64.zip` | 含 `cudart64_*.dll`、`cublas64_*.dll` |

直接下载链接模板（`<tag>` 替换为实际标签，如 `b6750`）：
```
https://github.com/ggml-org/llama.cpp/releases/download/<tag>/llama-<tag>-bin-win-cuda-12.4-x64.zip
https://github.com/ggml-org/llama.cpp/releases/download/<tag>/cudart-llama-bin-win-cuda-12.4-x64.zip
```
> **两个包都必须下载**。只下主程序包会“双击无响应 / 缺少 DLL”。40/50 系优先 **CUDA 12.4**（兼容性最成熟）。

### 3.2 解压目录结构
两包解压到**同一目录同一层**（不要嵌套）：
```
D:\llama-cpp\
  llama-server.exe  llama-cli.exe
  cudart64_12.dll   cublas64_12.dll   ...
```

### 3.3 获取 Q4_K_M 权重（单文件，无需 mmproj）
| 仓库 | 文件名 | 大小 | 说明 |
|------|--------|------|------|
| `ruygar/gemma-4-12B-it-GGUF` | `gemma-4-12b-it-Q4_K_M.gguf` | ~7.4 GB | 标注 Recommended |
| `AtomicChat/gemma-4-12B-it-GGUF` | 同 quant | ~7.2–7.4 GB | 自 bf16 量化 |
| `prithivMLmods/gemma-4-12B-it-GGUF` | `gemma-4-12B-it.Q4_K_M.gguf` | ~7.38 GB | 常见转换版 |

下载方式：
- 浏览器直接下载（可用迅雷加速）。
- HuggingFace CLI：`huggingface-cli download ruygar/gemma-4-12B-it-GGUF gemma-4-12b-it-Q4_K_M.gguf --local-dir D:\models`
- git-lfs：`git lfs install && git clone https://huggingface.co/ruygar/gemma-4-12B-it-GGUF D:\models\gemma4`

放置示例：`D:\models\gemma-4-12b-Q4_K_M.gguf`。

### 3.4 启动 llama-server（核心命令）
```powershell
.\llama-server.exe `
  -m D:\models\gemma-4-12b-Q4_K_M.gguf `
  -c 32768 -ngl 999 --jinja `
  --host 127.0.0.1 --port 8080 `
  --temp 1.0 --top-p 0.95 --top-k 64
```
**参数要点：**
- `-c 32768`：32K 上下文，显存约 **13.3 GB**（安全起点）；常 <8K 可改 `-c 8192`（约 9.4 GB）。
- `-ngl 999`：全部层卸载到 GPU（Gemma 4 12B 共 48 层）。
- `--jinja`：**关键**，使用 GGUF 内嵌的新 chat template，避免格式错乱。
- `--host 127.0.0.1`：仅本机访问，更安全。

### 3.5 一键启动脚本
`start-gemma-server.bat`（与 exe 同目录，双击即用），模型路径 / 上下文集中在顶部可改。详见文件。

### 3.6 Flash Attention
默认**不要**加 `-fa on`（Windows 预编译已知兼容问题，可能 CUDA illegal memory access）。稳定后可试加观察，报错即去掉。

### 3.7 Go 后端服务（前端统一入口，可选网关）
用 Go（chi/v5）实现的反代网关，作为前端统一入口，职责：
- **反向代理** llama-server 的 OpenAI 兼容 API：`/v1/chat/completions`（SSE 流式逐块透传）、`/v1/models`。前端无需直连 `:8080`。
- **托管前端**：`/chat` → `gemma-chat.html`，`/launcher` → `open-webui-example.html`。
- **健康检查** `/health` 同时探测上游；可选 API Key 鉴权、CORS。

文件（`backend/`）：`main.go`（路由/中间件/静态托管）、`proxy.go`（SSE 透传）、`config.go`（环境变量）、`go.mod`。已编译产物：`gemma-backend.exe`（Go 1.27 构建通过）。

运行：
```powershell
cd gemma4-deploy\backend
# 可选环境变量（均有默认值）：PORT=8081  UPSTREAM=http://127.0.0.1:8080  STATIC_DIR=..  API_KEY=（留空=不鉴权）
go run .          # 或直接使用 gemma-backend.exe
```
启动后入口：
- 前端：`http://localhost:8081/chat`、`http://localhost:8081/launcher`
- API：`http://localhost:8081/v1/chat/completions`
- 健康检查：`http://localhost:8081/health`

说明：`gemma-chat.html` 在经 http 访问时**自动**将 API Base 设为同源 `/v1`（即走 Go 后端）；直接双击以 `file://` 打开时回退直连 llama-server `:8080`。方案 A（Open WebUI）可直接连 `:8080`，也可把连接 URL 改为 `http://host.docker.internal:8081/v1` 走 Go 后端。

---

## 4. 图形化前端（两种方案）

### 4.1 方案 A：Open WebUI（功能最全，已选定）
自托管 AI 对话平台，原生支持 OpenAI 兼容后端，含多会话、历史、RAG、用户/权限管理。

**安装（Docker，Windows）**
```powershell
docker run -d -p 3000:8080 `
  --add-host=host.docker.internal:host-gateway `
  -v open-webui:/app/backend/data `
  -e WEBUI_SECRET_KEY=请改成你自己的强密钥 `
  --name open-webui --restart always `
  ghcr.io/open-webui/open-webui:main
```
> 关键 `--add-host=host.docker.internal:host-gateway`：让容器内访问到宿主机的 llama-server。`:main` 镜像 ≈1.6GB；纯聊天可换 `:slim`（≈196MB）。

**连接 llama-server（后端在 Windows 宿主）**
1. 打开 `http://localhost:3000`，首次**创建管理员账号**（强密码）。
2. ⚙️ **Admin Settings → Connections → OpenAI → ➕ Add Connection**。
3. URL：`http://host.docker.internal:8080/v1`（**不要写 localhost**，那是容器自己）。
4. API Key：`not-needed`。
5. 若连接校验报错（4xx）但聊天正常，在 **Model IDs (Filter)** 手动填 `gemma-4` 保存。
6. 返回聊天界面，顶部选 `gemma-4` 开始对话。

**安全**：创建管理员后建议 **关闭开放注册（Disable Signup）**。

**配套：`open-webui-example.html`（示例 + 启动台）**
- 单文件、无需 Docker，打开即用，含三标签：
  - **界面示例**：高保真还原 Open WebUI 布局（侧栏历史 / 模型选择器 / 聊天气泡 / 代码块），预置示例对话。
  - **真实 Open WebUI**：iframe 直连 `http://localhost:3000`（装好 Docker 后直接用）。
  - **安装与连接**：内置上面的一键命令与连接步骤。
- 右上角 `◐` 切换**浅色 / 深色**双主题，选择记忆于 `localStorage`。
- 顶栏自动探测 `:3000` 是否在运行。
- 用法：先装 Docker 并运行上面 `docker run`，再打开本文件切到「真实 Open WebUI」即可。

### 4.2 方案 B：gemma-chat.html（零依赖）
- 单文件前端，**支持流式输出**。
- 地址自适应：双击以 `file://` 打开时直连 `http://127.0.0.1:8080/v1`；经 Go 后端（`:8081`）托管访问时自动改为同源 `/v1`。
- 适合：快速验证部署、不想装 Docker、仅本机临时对话。
- 用法：先启动 llama-server（或 Go 后端），再打开本文件即可对话（可改 API Base / Key / Model / 采样）。

### 4.3 其他前端（简述）
- **Chatbox**（桌面 App）：设置 → 自定义模型 → OpenAI 兼容，填 `http://127.0.0.1:8080/v1`。
- **AnythingLLM**：偏多文档 RAG 知识库，同样接 OpenAI 兼容端点。
- **llama-server 内置 UI**：`http://localhost:8080` 极简聊天页，用于冒烟测试。

---

## 5. 验证部署

| 项 | 命令 / 操作 | 预期 |
|----|-------------|------|
| 健康检查 | `curl.exe http://127.0.0.1:8080/health` | HTTP 200（加载中 503 属正常） |
| 内置 UI 冒烟 | 浏览器开 `http://localhost:8080`，发“你好” | 正常回复 |
| OpenAI API | 见下 Python 片段 | 返回 completion |
| Open WebUI | 添加连接后选 `gemma-4` 发消息 | 正常对话 |

```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:8080/v1", api_key="not-needed")
r = client.chat.completions.create(
    model="gemma-4",
    messages=[{"role":"user","content":"用一句话解释什么是量化"}])
print(r.choices[0].message.content)
```
> `model` 字段被忽略，服务始终用启动加载的模型。

---

## 6. 性能与显存对照（RTX 4080 16GB，Q4_K_M）

| 上下文长度 | 显存占用（f16 KV Cache） | 预期速度 |
|------------|--------------------------|----------|
| 8K         | ~9.4 GB                  | 46–74 tok/s |
| 32K（默认）| ~13.3 GB                 | 46–65 tok/s |
| 125K+      | 超出 16GB，不适用        | — |

**是否正常**：32K 下若速度 **< 30 tok/s**，或 `nvidia-smi` 显示 `llama-server` 未占预期显存，优先查 `-ngl` 是否生效。

---

## 7. 采样参数
Google 推荐 `--temp 1.0 --top-p 0.95 --top-k 64`；严谨推理可将 `--temp` 降到 `0.3–0.7`。

---

## 8. 显存应急（按优先级）
1. **缩短上下文**：`-c 32768` → `-c 16384` 或 `-c 8192`（直接、零副作用）。
2. **量化 KV Cache**（显存约减半，质量影响通常很小）：加 `--cache-type-k q8_0 --cache-type-v q8_0`。

---

## 9. 故障速查

| 现象 | 最可能原因 | 处理 |
|------|------------|------|
| 双击无响应 / 缺 DLL | 漏掉 CUDA 运行库包 | 下载 `cudart-llama-bin-win-cuda-…-x64.zip` 同目录解压 |
| 启动闪退 / CUDA 报错 | 开了 `-fa on` | 去掉 `-fa` |
| 格式错乱 / 输出散文 | 缺 `--jinja` | 启动加 `--jinja` |
| 几轮后变慢/报错 | 上下文耗尽 | 确认 `-c`；查显存 |
| 显存不足 / 被挤出 | 上下文过长或他程序占显存 | 降 `-c`；加 `--cache-type-k/v q8_0` |
| 速度 < 30 tok/s | GPU 层未全卸载 | 确认 `-ngl 999`；`nvidia-smi` 看占用 |
| Open WebUI 连不上 | Docker 里写 `localhost` | 改用 `http://host.docker.internal:8080/v1` |
| Open WebUI 校验报错但能聊 | `/models` 探测非标准 | Model IDs (Filter) 手动加 `gemma-4` |

---

## 10. 运维与安全
- 后端 `--host 127.0.0.1` 仅本机；**勿**改 `0.0.0.0` 除非确需局域网共享且已配防火墙。
- Open WebUI 创建管理员后**关闭开放注册**。
- 权重较大，传输用移动硬盘/U盘；仓库一般提供 sha256，建议校验。
- 升级 llama.cpp：下载新版两个 zip 覆盖解压到同目录（保留 GGUF 与启动脚本）。
- Open WebUI 数据在 Docker 卷 `open-webui`（映射 `/app/backend/data`），重装容器数据不丢；备份即备份该卷。

---

## 11. 最短落地路径
```
下载 llama.cpp CUDA 主程序包 + 运行库包 → 同目录解压
下载 Q4_K_M 单 GGUF → D:\models\
运行 start-gemma-server.bat → llama-server @ :8080（推理后端）
（可选）cd backend && go run . → Go 后端 @ :8081（前端网关 + API 反代）
选前端：
  · 方案A：docker run Open WebUI → localhost:3000 连接 host.docker.internal:8081/v1（或 :8080/v1）
  · 方案B：浏览器开 http://localhost:8081/chat（或双击 gemma-chat.html）
```

---

## 12. 项目落地执行状态（本次实际落地）

> 本节记录“已自动完成 / 仍需手动”的真实状态，便于按图索骥。

### 12.1 已自动落地（代码就绪、已本地验证）
| 项 | 状态 | 说明 |
|----|------|------|
| Go 后端网关（chi/v5） | ✅ 已编译 `backend/gemma-backend.exe` | Go 1.27 编译通过；冒烟测试 `/health`、`/chat`、`/launcher`、`/` 302、`/v1/models` 均符合预期 |
| 前端托管 | ✅ 已验证 | `/chat` 返回 `gemma-chat.html`、`/launcher` 返回 `open-webui-example.html`（均 HTTP 200） |
| API 反代 + SSE 流式 | ✅ 实现 | `backend/proxy.go` 逐块 flush，前端流式逐字输出 |
| 前端 API Base 自适应 | ✅ 已修正 | 经 Go 网关(http)访问自动走同源 `/v1`；`file://` 直开回退直连 `:8080`（统一走网关，便于未来鉴权/限流） |
| 一键启动脚本 | ✅ `start-all.bat` | 拉起 Go 网关 + llama-server（两个独立窗口） |
| 环境自检脚本 | ✅ `check-env.bat` | 检查 GPU/驱动、llama.cpp 与权重、磁盘空间、端口占用 |

### 12.2 仍需你手动完成（沙箱无 GPU / 无外网权重，无法代下）
| 步骤 | 操作 | 完成后标志 |
|------|------|------------|
| 下载 llama.cpp CUDA 双包 | GitHub Releases 下载 `llama-<tag>-bin-win-cuda-12.4-x64.zip` + `cudart-llama-bin-win-cuda-12.4-x64.zip`，**同目录**解压到 `D:\llama-cpp\` | `D:\llama-cpp\llama-server.exe` 存在 |
| 下载 Q4_K_M 权重 | HuggingFace 取单文件 GGUF 到 `D:\models\gemma-4-12b-Q4_K_M.gguf`（约 7GB） | 文件存在且 sha256 一致 |
| （方案 A 专用）装 Docker | 安装 Docker Desktop（WSL2 后端） | `docker --version` 可用 |
| 启动推理后端 | 双击 `start-all.bat`（或先 `start-gemma-server.bat`） | LlamaServer 窗口出现 `llama_server listening`，`/health` 返回 ok |

### 12.3 推荐启动顺序
```
1) 双击 check-env.bat  → 确认环境（GPU / 权重 / 磁盘 / 端口）
2) 双击 start-all.bat   → 自动拉起 Go 网关(:8081) 与 llama-server(:8080 窗口)
3) 等待 LlamaServer 窗口 "llama_server listening"（模型加载完）
4) 浏览器打开 http://localhost:8081/chat  → 对话
   或（方案 A）docker run Open WebUI → localhost:3000 连接 http://host.docker.internal:8081/v1
```
> Go 网关启动后即使 llama-server 尚未就绪，也会返回 `degraded`（/health 探测上游），**无需重启网关**，上游就绪即自动转发。

---

## 13. 模型开关综合控制后台（admin）

> 用途：在本机用浏览器集中控制 llama-server 的**启 / 停 / 重启**、查看**运行状态**、**实时日志**与 **GPU 显存**，无需再手敲命令行或开多个窗口。

### 13.1 后端控制端点（由 Go 网关提供）
Go 后端（`backend/gemma-backend.exe`）新增 `/admin/*` 路由，用 `os/exec` 直接管理 llama-server 子进程：

| 端点 | 方法 | 作用 |
|------|------|------|
| `/admin/status` | GET | 返回 `state`（`stopped`/`starting`/`running`）、进程存活、上游就绪、当前配置（bin/权重/ctx/ngl） |
| `/admin/start` | POST | 按配置（或被 body 覆盖的 `model_path`/`ctx`/`ngl`）拉起 llama-server，收集 stdout/stderr 日志 |
| `/admin/stop` | POST | 结束 llama-server 子进程 |
| `/admin/restart` | POST | 先停后启 |
| `/admin/log` | GET | 返回 llama-server 输出日志（环形缓冲，前端轮询） |
| `/admin/gpu` | GET | 执行 `nvidia-smi` 返回 GPU 名称 / 显存 / 利用率 |

相关配置来自环境变量（默认见 `config.go`）：`LLAMA_BIN`、`MODEL_PATH`、`CTX`、`NGL`。

### 13.2 前端控制台 `admin.html`
- 简约风、浅/深色双主题（右上角 `◐` 切换，`localStorage` 记忆）。
- 经 Go 网关（http）访问自动连同源 `:8081`；`file://` 直开则连 `http://127.0.0.1:8081`。
- 功能：状态徽章（按状态自动禁用对应按钮）、启动参数表单（可覆盖权重/上下文/GPU 层）、实时日志、GPU 显存监控，全部每 1.5–5 秒自动刷新。
- 访问：`http://localhost:8081/admin`（由 `/admin` 路由托管）。

### 13.3 与启动脚本的关系
- `start-all.bat` 仍可用：它拉起 **Go 网关 + llama-server**（先起推理后端）。
- 用 admin 控制台时，**只需先启动 Go 网关**（`./backend/gemma-backend.exe`），模型的启停完全在 `:8081/admin` 页面里点按钮完成——更灵活，且能实时看加载日志与显存。
- 两种方式并存：直接双击 `start-gemma-server.bat` 起的 llama-server 不在 Go 管理内（admin 的“停止”管不到它），建议统一走 admin 或统一走 start-all。

---

## 附录 A：文件速查
- 一键启动（全链路）：`start-all.bat`
- 环境自检：`check-env.bat`
- 后端启动（仅推理）：`start-gemma-server.bat`
- 模型控制台（启停/状态/日志/显存）：`admin.html`（经 Go 网关 `:8081/admin` 访问）
- 前端（零依赖）：`gemma-chat.html`
- 前端（Open WebUI 启动台/示例）：`open-webui-example.html`
- 后端参数详参：`deploy-guide.md`

## 附录 B：常用端口
- `127.0.0.1:8080` — llama-server（推理后端 + 内置 UI + OpenAI API）
- `localhost:3000` — Open WebUI（方案 A 前端）
