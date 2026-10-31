# Gemma 4 12B Windows 桌面端综合控制后台落地开发文档

## 1. 目标

本项目的运行时由三个部分组成：

1. `llama-server.exe`：加载 GGUF 模型并使用 CUDA 推理。
2. Go 网关：提供 OpenAI 兼容 API、前端托管和模型进程控制。
3. Windows 桌面启动器：后台启动 Go 网关，调用模型控制接口，并打开独立的桌面应用窗口。

桌面端的主入口是模型控制后台，而不是普通浏览器聊天页。用户可以在控制台中完成：

- 启动、停止、重启模型；
- 切换 GGUF 权重；
- 修改上下文长度和 GPU 卸载层数；
- 查看 llama-server 实时日志；
- 查看上游健康状态、进程状态和 RTX 显存；
- 从控制台进入聊天页面。

聊天会话保存在浏览器本地存储中。桌面端和 `http://127.0.0.1:8081/chat` 使用同一来源，因此重启程序后可以继续之前的会话；清除 Edge 网站数据或更换 Windows 用户后，历史记录不会自动迁移。

## 2. 运行架构

```text
gemma-desktop.exe
    |
    +-- 启动 backend/gemma-backend.exe (:8081)
    |       |
    |       +-- /admin 控制台
    |       +-- /admin/start|stop|restart 模型控制
    |       +-- /admin/status|log|gpu 状态查询
    |       +-- /v1/* OpenAI 兼容反代
    |
    +-- backend 统一管理 llama-server.exe (:8080)
            |
            +-- D:\models\*.gguf
            +-- RTX 4080 CUDA
```

模型进程只能由 Go 后端统一管理。不要再从外部单独启动 `llama-server.exe`，否则后台无法准确显示进程归属，也无法保证“停止”和“重启”操作作用于正确的实例。

## 3. 整理后的文件结构

```text
gemma4-deploy/
├─ backend/                         Go 控制服务源码
│  ├─ main.go                       路由、静态托管、API 反代
│  ├─ admin.go                      模型启停、日志、GPU 控制
│  ├─ config.go                     环境变量和默认配置
│  ├─ proxy.go                      SSE 反向代理
│  ├─ go.mod
│  └─ README.md
├─ desktop/                         Windows 桌面启动器源码
│  ├─ main.go
│  └─ go.mod
├─ frontend/                        HTML/CSS/JavaScript 前端
│  ├─ admin.html                    综合模型控制台
│  ├─ gemma-chat.html               本地聊天页
│  └─ open-webui-example.html       Open WebUI 示例和安装台
├─ scripts/                         运维和构建脚本
│  ├─ start-all.bat                 一键启动后台和控制台
│  ├─ start-chat.bat                聊天桌面端启动
│  ├─ start-gemma-server.bat        手动启动模型，调试用途
│  ├─ check-env.bat                 环境检查
│  └─ build-desktop.ps1             构建桌面端
├─ docs/                            项目文档
│  ├─ windows-desktop-admin-development.md
│  ├─ Gemma4-12B-综合部署落地文档.md
│  └─ deploy-guide.md
└─ dist/                            构建产物，不放源码
   ├─ gemma-desktop.exe
   ├─ gemma-chat.exe
   └─ gemma-backend.exe
```

模型和 CUDA 运行时不纳入项目仓库：

```text
D:\models\gemma-4-12b-Q4_K_M.gguf
D:\llama-cpp\llama-server.exe
D:\llama-cpp\*.dll
```

## 4. 控制状态机

后台使用单模型、单端口策略：

```text
STOPPED --/admin/start--> STARTING --health 200--> RUNNING
   ^                         |                       |
   +------/admin/stop--------+--------/admin/stop---+
                             |
                         process exit
                             |
                          STOPPED
```

### 4.1 模型切换

模型切换不是并行加载。后台必须按以下顺序执行：

1. 停止当前 llama-server；
2. 等待 `127.0.0.1:8080/health` 不再可用；
3. 校验新 GGUF 路径存在；
4. 使用新路径、上下文和 GPU 层数启动；
5. 等待健康检查恢复 200；
6. 更新控制台状态和日志。

单张 RTX 4080 只有一个 8080 推理端口，禁止两个模型同时占用该端口。

## 5. HTTP 控制接口

| 方法 | 路径 | 作用 |
|---|---|---|
| GET | `/admin` | 控制台页面 |
| GET | `/admin/status` | 进程、健康状态和当前配置 |
| POST | `/admin/start` | 启动模型，可传 `model_path`、`ctx`、`ngl` |
| POST | `/admin/stop` | 停止当前模型 |
| POST | `/admin/restart` | 停止后重新启动 |
| GET | `/admin/log` | 最近模型日志 |
| GET | `/admin/gpu` | `nvidia-smi` 显存和利用率 |
| GET | `/health` | Go 网关及上游健康状态 |
| POST | `/v1/chat/completions` | OpenAI 兼容聊天 API |

示例启动请求：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/start `
  -Method Post `
  -ContentType 'application/json' `
  -Body '{"model_path":"D:\\models\\gemma-4-12b-Q4_K_M.gguf","ctx":"16384","ngl":"999"}'
```

## 6. 桌面端启动流程

双击 `dist\gemma-desktop.exe` 后：

1. 检查后端、模型和 llama-server 文件；
2. 后台启动 Go 网关；
3. 等待 `/admin/status` 可访问；
4. 若模型未运行，调用 `/admin/start`；
5. 等待模型健康检查通过；
6. 使用 Microsoft Edge `--app` 模式打开 `http://127.0.0.1:8081/admin`。

双击 `dist\gemma-chat.exe` 会执行相同的前后台启动流程，但最后打开 `http://127.0.0.1:8081/chat` 聊天窗口。

该方案是桌面应用窗口，不会打开普通浏览器标签页。它使用系统已有的 Edge 渲染 HTML，不需要 .NET SDK。若未来要求完全内嵌且不依赖 Edge，可再迁移到 WebView2 或 Wails，但那属于第二阶段打包工作。

## 7. 构建和启动

在项目根目录执行：

```powershell
Set-Location E:\MyAi\gemma4-deploy
Set-Location .\backend
go build -o ..\dist\gemma-backend.exe .
Set-Location ..
.\scripts\build-desktop.ps1
```

启动桌面端：

```powershell
.\dist\gemma-desktop.exe
```

传统脚本入口仍保留：

```powershell
.\scripts\check-env.bat
.\scripts\start-all.bat
.\scripts\start-chat.bat
```

开发调试时直接访问：

```text
http://127.0.0.1:8081/admin
http://127.0.0.1:8081/chat
```

## 8. 部署验收

### 文件验收

```powershell
Test-Path D:\llama-cpp\llama-server.exe
Test-Path D:\models\gemma-4-12b-Q4_K_M.gguf
Test-Path .\dist\gemma-desktop.exe
Test-Path .\dist\gemma-chat.exe
Test-Path .\dist\gemma-backend.exe
```

### 服务验收

```powershell
curl.exe http://127.0.0.1:8081/admin/status
curl.exe http://127.0.0.1:8081/admin/gpu
curl.exe http://127.0.0.1:8080/health
```

### 控制验收

1. 桌面端启动后，控制台显示“运行中”。
2. 点击“停止”，8080 健康检查失败，状态变为“停止”。
3. 点击“启动”，状态经历“启动中”后恢复“运行中”。
4. 修改模型路径后点击“重启”，日志出现新的启动参数。
5. 点击聊天入口，能够完成流式回复。
6. GPU 面板能够显示显存使用量。

## 9. 安全与运维边界

- 默认仅监听 `127.0.0.1`，不提供局域网访问。
- `/admin` 具备启动本机进程能力，不应直接暴露到公网。
- 如需局域网使用，必须增加 API Key、来源限制和 Windows 防火墙规则。
- 模型路径输入应限制在允许的模型目录内；生产版不应允许任意可执行文件路径。
- 日志只保留最近 2000 行，避免长时间运行导致内存增长。
- 同一时间只允许一个 llama-server 子进程归后台所有。

## 10. 后续迭代

第一阶段已经覆盖单机单模型控制。后续可以增加：

- 模型列表扫描和下拉选择；
- 多模型配置文件与收藏参数；
- 进程树级停止，避免子进程残留；
- 桌面系统托盘、最小化和退出时清理服务；
- WebView2/Wails 真正内嵌窗口；
- 安装包、快捷方式和卸载程序；
- 访问鉴权和操作审计日志。
