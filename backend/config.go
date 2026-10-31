package main

import "os"

// Config 通过环境变量加载，方便不同机器/部署调整。
type Config struct {
	Port        string // 本 Go 服务监听端口，默认 8081
	Upstream    string // llama-server 地址，默认 http://127.0.0.1:8080
	UpstreamKey string // 访问 llama-server 用的 Key（llama-server 不校验，默认 not-needed）
	Model       string // 对外暴露的模型名，默认 gemma-4
	StaticDir   string // 前端 HTML 所在目录，默认 "../frontend"
	APIKey      string // 可选：客户端访问本 Go 服务所需的 Key，留空表示不鉴权
	LlamaBin    string // llama-server 可执行文件绝对路径
	ModelDir    string // 允许加载的 GGUF 根目录
	ModelPath   string // GGUF 权重绝对路径
	Ctx         string // 上下文长度（默认 32768）
	NGL         string // GPU 卸载层数（默认 999 = 全部）
}

func loadConfig() *Config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return &Config{
		Port:        get("PORT", "8081"),
		Upstream:    get("UPSTREAM", "http://127.0.0.1:8080"),
		UpstreamKey: get("UPSTREAM_API_KEY", "not-needed"),
		Model:       get("MODEL", "gemma-4"),
		StaticDir:   get("STATIC_DIR", "../frontend"),
		APIKey:      get("API_KEY", ""),
		LlamaBin:    get("LLAMA_BIN", `D:\llama-cpp\llama-server.exe`),
		ModelDir:    get("MODEL_DIR", `D:\models`),
		ModelPath:   get("MODEL_PATH", `D:\models\gemma-4-12b-Q4_K_M.gguf`),
		Ctx:         get("CTX", "32768"),
		NGL:         get("NGL", "999"),
	}
}
