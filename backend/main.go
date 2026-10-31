package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server 持有配置与复用 HTTP 客户端（流式场景不设总超时）。
type Server struct {
	cfg    *Config
	client *http.Client

	// admin 控制相关
	mu     sync.Mutex
	cmd    *exec.Cmd // 当前管理的 llama-server 子进程
	logBuf []string  // llama-server 输出日志环形缓冲
}

const maxLogLines = 2000

func NewServer(cfg *Config) *Server {
	return &Server{
		cfg:    cfg,
		client: &http.Client{}, // Timeout=0：不限制总时长，支持长流式输出
		logBuf: make([]string, 0, maxLogLines),
	}
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)
	if s.cfg.APIKey != "" {
		r.Use(s.apiKeyMiddleware)
	}

	// 健康检查：同时探测 llama-server 上游
	r.Get("/health", s.handleHealth)

	// OpenAI 兼容 API 透传
	r.Get("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		s.proxy(w, r, "/v1/models")
	})
	r.Post("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		s.proxy(w, r, "/v1/chat/completions")
	})

	// 模型开关综合控制（admin）
	r.Get("/admin/status", s.handleAdminStatus)
	r.Post("/admin/start", s.handleAdminStart)
	r.Post("/admin/stop", s.handleAdminStop)
	r.Post("/admin/restart", s.handleAdminRestart)
	r.Get("/admin/log", s.handleAdminLog)
	r.Get("/admin/gpu", s.handleAdminGPU)

	// 前端 HTML 托管
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/chat", http.StatusFound)
	})
	r.Get("/chat", s.serveFile("gemma-chat.html"))
	r.Get("/launcher", s.serveFile("open-webui-example.html"))
	r.Get("/admin", s.serveFile("admin.html"))
	r.Get("/static/*", s.staticFiles())

	return r
}

// handleHealth 检查本服务及其上游 llama-server 是否可用。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp, err := s.client.Get(s.cfg.Upstream + "/health")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"degraded","upstream":"%s","error":"%v"}`, s.cfg.Upstream, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		fmt.Fprintf(w, `{"status":"ok","upstream":"%s"}`, s.cfg.Upstream)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, `{"status":"degraded","upstream_status":%d}`, resp.StatusCode)
}

func (s *Server) serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := readStatic(s.cfg.StaticDir, name)
		if err != nil {
			http.Error(w, "file not found: "+name, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func (s *Server) staticFiles() http.HandlerFunc {
	fs := http.FileServer(http.Dir(s.cfg.StaticDir))
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/static")
		fs.ServeHTTP(w, r)
	}
}

func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if key != s.cfg.APIKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware 允许前端（含 file:// 直接打开的页面）跨域调用本服务。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func readStatic(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}

func main() {
	cfg := loadConfig()
	s := NewServer(cfg)
	addr := ":" + cfg.Port

	log.Printf("Gemma Go backend 监听 %s  (代理 -> %s)", addr, cfg.Upstream)
	log.Printf("前端入口:  http://localhost%s/chat   http://localhost%s/launcher   http://localhost%s/admin", addr, addr, addr)
	log.Printf("API:       http://localhost%s/v1/chat/completions", addr)
	log.Printf("控制:      http://localhost%s/admin  (模型启停/状态/日志/显存)", addr)

	if err := http.ListenAndServe(addr, s.routes()); err != nil {
		log.Fatal(err)
	}
}
