# Gemma 4 12B 本地部署落地方案（详细版）

> 基础文档：原《Gemma 4 12B 本地部署落地文档》
> 本方案在其基础上**扩展细节、补充一键脚本、并增加「前台图形化界面」配置**。
> **已选定前端：方案 A（Open WebUI）**——功能最全（多会话 / 历史 / RAG / 用户管理）。详见第 5 节与配套示例启动台 `open-webui-example.html`。
> 目标硬件：NVIDIA GeForce RTX 4080 16GB｜模型格式：GGUF Q4_K_M｜后端：llama.cpp（llama-server）｜场景：纯文本对话 / 推理，不加载多模态投影器。

---

## 0. 架构总览

```
┌─────────────────────────────────────────────────────────────┐
│                        Windows 主机                           │
│                                                               │
│   [ 前台图形化界面 ]                         [ llama-server ]  │
│   ┌───────────────────────┐                ┌──────────────┐  │
│   │ 方案A: Open WebUI      │   HTTP         │ llama.cpp    │  │
│   │   (Docker :3000)      │ ─────────────▶ │ CUDA 推理    │  │
│   │ 方案B: gemma-chat.html│  OpenAI 兼容    │ RTX 4080     │  │
│   │   (零依赖 本地文件)    │   /v1 API      │ 16GB 显存    │  │
│   └───────────────────────┘                └──────┬───────┘  │
│                                                    │          │
│                                            D:\models\        │
│                                            gemma-4-12b-      │
│                                            Q4_K_M.gguf       │
└─────────────────────────────────────────────────────────────┘
```

- **后端**：`llama-server.exe` 监听 `127.0.0.1:8080`，对外暴露 OpenAI 兼容 API（`/v1/chat/completions`、`/v1/models`、`/health`）。
- **前端**：任选其一。方案 A（Open WebUI）功能最全（多会话、历史、RAG、用户管理）；方案 B（自带 `gemma-chat.html`）零安装、双击即用，适合快速验证。

---

## 1. 部署前确认（逐项核对）

### 1.1 GPU 与驱动
```powershell
nvidia-smi
```
确认：能看到 `RTX 4080`，且右上角 `CUDA Version` 正常显示（≥ 12.4 即可，你的驱动 `32.0.16.1088` 已足够新）。
> 注意：这里显示的是**驱动支持的最高 CUDA 版本**，不是必须安装 CUDA Toolkit。llama.cpp 的 CUDA 预编译包自带所需运行库（cudart/cublas DLL），无需单独安装 Toolkit。

### 1.2 磁盘空间
- Q4_K_M 权重约 **6.9 – 7.4 GB**（不同转换来源略有差异）。
- 预留 **10 GB 以上** 空闲空间（含临时文件与日志）。
- 建议路径简短、无空格、无中文，例如：`D:\models\`、`D:\llama-cpp\`。

### 1.3 系统内存
- **16 GB** 可运行；但 Windows 后台程序 + 模型加载峰值会占用一部分，加载与切换略紧。
- **32 GB** 体验明显更从容（非硬性门槛）。

### 1.4（新增）网络与运行环境
- 能访问 `github.com`（下载 llama.cpp / 权重）。
- 若选 Open WebUI：需已安装 **Docker Desktop**（Windows 版，开启 WSL2 后端）。
- 若用自带 HTML 前端：只需任意现代浏览器（Chrome / Edge / Firefox），无需联网到外网。

---

## 2. 获取 llama-server（Windows 预编译，免编译）

> 最易“第一步卡住”的环节。**不要从源码编译**，直接下载官方预编译 CUDA 版本。

### 2.1 下载正确的两个包
打开 llama.cpp Releases 页面：
`https://github.com/ggml-org/llama.cpp/releases`

找到 **Latest** 版本（标签形如 `bXXXX`，例如 `b6750`）。在同一次 release 的 Assets 列表中下载**两个**文件（注意：包名里的版本号就是 release 标签，下面用 `<tag>` 占位，请替换成实际最新标签）：

| 文件 | 命名特征 | 用途 |
|------|----------|------|
| 主程序包 | `llama-<tag>-bin-win-cuda-12.4-x64.zip` | 含 `llama-server.exe`、`llama-cli.exe` 等 |
| CUDA 运行库 | `cudart-llama-bin-win-cuda-12.4-x64.zip` | 含 `cudart64_*.dll`、`cublas64_*.dll` 等必需 DLL |

直接下载链接模板（把 `<tag>` 替换为实际标签，如 `b6750`）：
```
https://github.com/ggml-org/llama.cpp/releases/download/<tag>/llama-<tag>-bin-win-cuda-12.4-x64.zip
https://github.com/ggml-org/llama.cpp/releases/download/<tag>/cudart-llama-bin-win-cuda-12.4-x64.zip
```
> **两个包都必须下载。** 只下主程序包、漏掉 CUDA DLL 包，是“双击无响应 / 缺少 DLL”的最常见原因。
> 40/50 系显卡优先选 **CUDA 12.4** 预编译包（兼容性记录更成熟）；若你看到 13.x 包也可用，但 12.4 优先。

### 2.2 解压与目录结构
将两个压缩包**解压到同一目录的同一层**（不要嵌套子文件夹），例如 `D:\llama-cpp\`：
```
D:\llama-cpp\
  llama-server.exe
  llama-cli.exe
  cudart64_12.dll
  cublas64_12.dll
  ...
```
Windows 会从 `llama-server.exe` 所在目录加载 DLL，因此两者必须在同一层。

### 2.3（可选）加入 PATH
若希望在任何目录都能调用，可将 `D:\llama-cpp\` 加入系统 `PATH` 环境变量。不加入也完全可用——只要在启动时 `cd` 到该目录即可。

---

## 3. 获取 Q4_K_M 权重（单文件）

纯文本场景**只需一个 GGUF 文件**，不需要任何 `mmproj` / 投影器文件。推荐仓库（任选其一，均提供 Q4_K_M）：

| 仓库 | 文件名 | 大小 | 说明 |
|------|--------|------|------|
| `ruygar/gemma-4-12B-it-GGUF` | `gemma-4-12b-it-Q4_K_M.gguf` | ~7.4 GB | 标注 Recommended，中端 GPU 甜点选择 |
| `AtomicChat/gemma-4-12B-it-GGUF` | 同 quant 文件名 | ~7.2–7.4 GB | 自 Google bf16 权重量化，兼容 llama.cpp |
| `prithivMLmods/gemma-4-12B-it-GGUF` | `gemma-4-12B-it.Q4_K_M.gguf` | ~7.38 GB | 常见转换版本 |

下载方式（任选）：
- **浏览器**：在 HuggingFace 对应仓库页面直接点击文件下载（可用迅雷等工具加速大文件）。
- **HuggingFace CLI**（需 Python 环境）：
  ```powershell
  pip install -U "huggingface_hub[cli]"
  huggingface-cli download ruygar/gemma-4-12B-it-GGUF gemma-4-12b-it-Q4_K_M.gguf --local-dir D:\models
  ```
- **git + git-lfs**（仓库较大时）：
  ```powershell
  git lfs install
  git clone https://huggingface.co/ruygar/gemma-4-12B-it-GGUF D:\models\gemma4
  ```

放置到例如：`D:\models\gemma-4-12b-Q4_K_M.gguf`（路径无空格、无中文）。

---

## 4. 启动 llama-server

### 4.1 核心命令（PowerShell）
进入 `llama-server.exe` 所在目录后执行：
```powershell
.\llama-server.exe `
  -m D:\models\gemma-4-12b-Q4_K_M.gguf `
  -c 32768 `
  -ngl 999 `
  --jinja `
  --host 127.0.0.1 `
  --port 8080 `
  --temp 1.0 --top-p 0.95 --top-k 64
```
**逐项解释：**
- `-m`：GGUF 绝对路径。
- `-c 32768`：上下文 32K。Q4_K_M 在 32K 下显存约 **13.3 GB**，仍在 16GB 内，是安全实用的起点。若对话常 < 8K，可用 `-c 8192`（显存约 9.4 GB，余量更大）。
- `-ngl 999`：尽可能多地把层卸载到 GPU；`999` 是“全部卸载”的约定值，会让 Gemma 4 12B 的全部 48 层进入显存。
- `--jinja`：**关键**。Gemma 4 使用新 chat template（``<|turn>`` / ``<|/turn>``），与 Gemma 1/2/3 内置模板不同。GGUF 内嵌正确 Jinja 模板，`--jinja` 让 llama.cpp 使用它，避免格式错乱 / 工具调用路由失败。
- `--host 127.0.0.1 --port 8080`：监听本地 8080，浏览器 UI 与 OpenAI 兼容 API 均经此访问。绑定到 `127.0.0.1` 即**仅本机可访问**，更安全。
- 末尾采样参数：Google 推荐 `--temp 1.0 --top-p 0.95 --top-k 64`；严谨推理可把 `--temp` 降到 `0.3–0.7`。

### 4.2 关于 Flash Attention：默认不要开启
Windows CUDA 预编译版中 `-fa on` 存在已知兼容问题（可能触发 `CUDA illegal memory access` 或 `cudaErrorSharedObjectInitFailed`）。纯文本 Q4_K_M 场景即使能启动收益也有限，而崩溃是实打实的。**首次部署不加 `-fa`**；稳定后可试加 `-fa on` 观察，报错即去掉。

### 4.3（新增）一键启动脚本
已提供 `start-gemma-server.bat`（放在 `D:\llama-cpp\` 与 exe 同目录），双击即启动 32K + 全 GPU 卸载 + jinja 的生产配置：
```bat
@echo off
.\llama-server.exe ^
  -m D:\models\gemma-4-12b-Q4_K_M.gguf ^
  -c 32768 ^
  -ngl 999 ^
  --jinja ^
  --host 127.0.0.1 ^
  --port 8080 ^
  --temp 1.0 --top-p 0.95 --top-k 64
```
> 若需改为 8K 或加 KV Cache 量化，编辑对应行即可（见第 8 节）。

---

## 5. 配置前台图形化界面（重点新增）

后端已在 `:8080` 提供 OpenAI 兼容 API。下面给出**两种**前端方案。

### 方案 A：Open WebUI（功能最全，推荐日常使用）
Open WebUI 是自托管的 AI 对话平台，原生支持 OpenAI 兼容后端，自带多会话管理、历史、RAG、用户/权限管理。

**5A.1 安装（Docker，Windows）**
需先安装 Docker Desktop（开启 WSL2）。管理员 PowerShell 执行：
```powershell
docker run -d -p 3000:8080 `
  --add-host=host.docker.internal:host-gateway `
  -v open-webui:/app/backend/data `
  -e WEBUI_SECRET_KEY=请改成你自己的强密钥 `
  --name open-webui --restart always `
  ghcr.io/open-webui/open-webui:main
```
- `-p 3000:8080`：浏览器访问 `http://localhost:3000`。
- `--add-host=host.docker.internal:host-gateway`：**关键**，让容器内才能访问到宿主机（Windows）上的 `llama-server`。
- 首次启动镜像较大（`:main` ≈ 1.6 GB）；若仅需纯聊天、想更小体积，可改用 `:slim`（≈ 196 MB，依赖外部服务做嵌入/文档抽取）。

**5A.2 连接 llama-server（后端在 Windows 宿主）**
1. 浏览器打开 `http://localhost:3000`，首次访问**创建管理员账号**（设强密码）。
2. 进入 ⚙️ **Admin Settings → Connections → OpenAI**（或直接 Settings → 管理员 → 连接）。
3. 点击 **➕ Add Connection**，填写：
   - **URL**：`http://host.docker.internal:8080/v1`（容器内能解析到宿主机的 llama-server；**不要写 localhost**，那是容器自己）
   - **API Key**：`not-needed`（llama-server 不校验，任意非空串均可）
4. 若连接校验报错（如 4xx）但聊天正常，属正常现象：llama-server 的 `/models` 探测偶尔返回非标准响应。此时在连接设置的 **Model IDs (Filter)** 手动填入 `gemma-4` 并保存，模型即可在选择器出现。
5. 返回聊天界面，顶部模型选择器选 `gemma-4`，开始对话。

> 安全提示：若仅在自己电脑使用，建议在 Admin Settings 中**关闭开放注册（Disable Signup）**，避免同局域网他人误登。

### 方案 B：gemma-chat.html（零依赖，双击即用）
已随本方案提供 `gemma-chat.html`，一个**单文件、无外部依赖**的本地聊天界面，直接调用 `:8080` 的 OpenAI 兼容 API，**支持流式输出**。

**5B.1 使用**
1. 先按第 4 节启动 `llama-server`（确保 `:8080` 已就绪）。
2. 双击 `gemma-chat.html` 用浏览器打开。
3. 默认已填：`API Base = http://127.0.0.1:8080/v1`、`API Key = not-needed`、`Model = gemma-4`。如需改上下文/采样，可在界面“高级”区调整。
4. 直接输入消息即可对话，回复逐字流式显示。

**5B.2 适用场景**
- 快速验证部署是否成功（最简 GUI）。
- 不想装 Docker、不想暴露端口、只想在本机临时对话。
- 可随 U 盘/内网分发，纯前端、无安装。

### 其他可选前端（简要）
- **Chatbox**（桌面 App）：设置 → 添加自定义模型 → 选 OpenAI 兼容，填 `http://127.0.0.1:8080/v1`。
- **AnythingLLM**：多文档 RAG 向，适合做知识库问答，同样接 OpenAI 兼容端点。
- **llama-server 内置 UI**：`http://localhost:8080` 自带极简聊天页，可用于冒烟测试（非长期主力）。

---

## 6. 验证部署

### 6.1 健康检查
另开 PowerShell：
```powershell
curl.exe http://127.0.0.1:8080/health
```
返回 HTTP 200（或健康 JSON）即就绪。模型加载期间可能返回 503，稍等再试。

### 6.2 浏览器 UI 冒烟
打开 `http://localhost:8080`，发一句“你好”确认能回复。

### 6.3 OpenAI 兼容 API
```python
from openai import OpenAI
client = OpenAI(base_url="http://127.0.0.1:8080/v1", api_key="not-needed")
resp = client.chat.completions.create(
    model="gemma-4",
    messages=[{"role": "user", "content": "用一句话解释什么是量化"}],
)
print(resp.choices[0].message.content)
```
> `model` 字段内容会被忽略，服务始终使用启动时加载的模型。

### 6.4 Open WebUI 连接验证
按 5A.2 添加连接后，在聊天界面选择 `gemma-4` 发送测试消息；若列表不显示模型，按 5A.2 第 4 步手动加 Model ID。

---

## 7. 性能预期与显存对照（RTX 4080 16GB，Q4_K_M）

| 上下文长度 | 显存占用（f16 KV Cache） | 预期速度（参考） |
|------------|--------------------------|------------------|
| 8K         | ~9.4 GB                  | 46 – 74 tok/s    |
| 32K（默认推荐） | ~13.3 GB             | 46 – 65 tok/s    |
| 125K+      | 超出 16GB，不适用        | —                |

> 速度来自多方估算与实测，46–74 tok/s 为合理区间，受采样参数、批处理、系统负载影响。

**是否正常判断**：32K 上下文中，若生成速度 **< 30 tok/s**，或 `nvidia-smi` 显示 `llama-server` 未占用预期显存，说明 GPU 卸载不完整或参数有误，**优先检查 `-ngl` 是否生效**。

---

## 8. 显存紧张时的应急调整（按优先级）

1. **缩短上下文**（最直接、零副作用）：`-c 32768` → `-c 16384` 或 `-c 8192`。
2. **量化 KV Cache**（显存约减半，质量影响通常很小）：在启动命令加
   ```
   --cache-type-k q8_0 --cache-type-v q8_0
   ```
   将 KV Cache 从 f16 降到 q8_0，是 16GB 卡释放余量的可靠手段。

---

## 9. 故障速查

| 现象 | 最可能原因 | 处理 |
|------|------------|------|
| 双击无响应 / 报“缺少 DLL” | 只下主程序包，漏掉 CUDA 运行库包 | 下载 `cudart-llama-bin-win-cuda-…-x64.zip`，与 exe 解压到同一目录 |
| 启动后闪退 / CUDA 报错 | 开启了 `-fa on` | 去掉 `-fa`（Windows 预编译已知兼容问题） |
| 对话格式错乱、输出 Python 散文 | 缺少 `--jinja` | 启动命令加 `--jinja` |
| 对话几轮后变慢/报错 | 上下文耗尽 | 确认 `-c` 是否足够；检查显存余量 |
| 显存不足 / 模型被挤出 | 上下文过长或其他程序占显存 | 降低 `-c`；加 `--cache-type-k q8_0 --cache-type-v q8_0` |
| 速度明显 < 30 tok/s | GPU 层未完全卸载 | 确认 `-ngl 999`；`nvidia-smi` 看是否占 GPU |
| Open WebUI 连不上后端 | Docker 里写了 `localhost` | 改用 `http://host.docker.internal:8080/v1` |
| Open WebUI 连接校验报错但能聊 | `/models` 探测非标准 | 在 Model IDs (Filter) 手动加 `gemma-4` |

---

## 10. 最短路径（速查）

```
下载 llama.cpp CUDA 主程序包 + CUDA 运行库包
  → 解压到同一目录（如 D:\llama-cpp\）
下载 Q4_K_M 单 GGUF（如 D:\models\gemma-4-12b-Q4_K_M.gguf）
  → 运行 start-gemma-server.bat（或手动命令）
  → 浏览器开 localhost:8080 冒烟
  → 选前端：Open WebUI（:3000）或 gemma-chat.html
```

---

## 11. 安全与运维提示
- 后端 `--host 127.0.0.1` 仅本机可访问；**不要**改为 `0.0.0.0` 除非确需局域网共享且已配防火墙。
- Open WebUI 首次创建管理员账号后，**关闭开放注册**。
- 权重文件较大，建议校验下载完整性（仓库一般提供 sha256）；传输到其它机器用移动硬盘/U盘而非反复下载。
- 升级 llama.cpp：下载更新版本的两个 zip，覆盖解压到同一目录即可（保留你的 GGUF 与启动脚本不动）。
