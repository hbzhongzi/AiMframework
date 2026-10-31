# AiMframework

Gemma 4 12B 本地部署与 Windows 桌面控制项目。

## 目录

- `backend/` Go API 网关和模型进程控制
- `desktop/` Windows 桌面启动器源码
- `frontend/` 聊天页和模型控制台
- `scripts/` 启动、检查和构建脚本
- `docs/` 部署、桌面控制和 API 文档
- `dist/` 已构建的 Windows EXE

## 启动

模型文件：`D:\models\gemma-4-12b-Q4_K_M.gguf`

llama.cpp：`D:\llama-cpp\llama-server.exe`

控制后台：

```powershell
.\scripts\start-all.bat
```

聊天桌面端：

```powershell
.\scripts\start-chat.bat
```

也可以直接运行 `dist\gemma-desktop.exe` 或 `dist\gemma-chat.exe`。

## 文档

- `docs/windows-desktop-admin-development.md`
- `docs/backend-api-reference.md`
- `docs/Gemma4-12B-综合部署落地文档.md`
