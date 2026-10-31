package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// adminStartReq 是 /admin/start 的可选覆盖参数。
type adminStartReq struct {
	ModelPath string `json:"model_path"`
	Ctx       string `json:"ctx"`
	NGL       string `json:"ngl"`
}

func writeJSON(w http.ResponseWriter, m map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) upstreamReady() bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(s.cfg.Upstream + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Server) waitUpstreamDown(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !s.upstreamReady() {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func (s *Server) validateStart(req adminStartReq) (string, string, string, error) {
	modelPath := s.cfg.ModelPath
	ctxText := s.cfg.Ctx
	nglText := s.cfg.NGL
	if req.ModelPath != "" {
		modelPath = req.ModelPath
	}
	if req.Ctx != "" {
		ctxText = req.Ctx
	}
	if req.NGL != "" {
		nglText = req.NGL
	}

	modelPath, err := filepath.Abs(modelPath)
	if err != nil {
		return "", "", "", fmt.Errorf("模型路径无效: %w", err)
	}
	root, err := filepath.Abs(s.cfg.ModelDir)
	if err != nil {
		return "", "", "", fmt.Errorf("模型目录无效: %w", err)
	}
	rel, err := filepath.Rel(root, modelPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", "", fmt.Errorf("模型必须位于允许目录: %s", root)
	}
	if !strings.EqualFold(filepath.Ext(modelPath), ".gguf") {
		return "", "", "", fmt.Errorf("模型文件必须是 .gguf")
	}
	stat, err := os.Stat(modelPath)
	if err != nil || stat.IsDir() {
		return "", "", "", fmt.Errorf("模型文件不存在: %s", modelPath)
	}

	ctx, err := strconv.Atoi(ctxText)
	if err != nil || ctx < 512 || ctx > 131072 {
		return "", "", "", fmt.Errorf("ctx 必须是 512 到 131072 之间的整数")
	}
	ngl, err := strconv.Atoi(nglText)
	if err != nil || ngl < 0 || ngl > 9999 {
		return "", "", "", fmt.Errorf("ngl 必须是 0 到 9999 之间的整数")
	}
	return modelPath, strconv.Itoa(ctx), strconv.Itoa(ngl), nil
}

func (s *Server) appendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logBuf = append(s.logBuf, line)
	if len(s.logBuf) > maxLogLines {
		s.logBuf = s.logBuf[len(s.logBuf)-maxLogLines:]
	}
}

// handleAdminStatus 返回进程存活 + 上游就绪 + 当前配置。
func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	procAlive := s.cmd != nil && s.cmd.Process != nil
	s.mu.Unlock()

	ready := s.upstreamReady()

	state := "stopped"
	if procAlive {
		if ready {
			state = "running"
		} else {
			state = "starting"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"state":        state,
		"process":      procAlive,
		"upstream":     ready,
		"llama_bin":    s.cfg.LlamaBin,
		"model_path":   s.cfg.ModelPath,
		"ctx":          s.cfg.Ctx,
		"ngl":          s.cfg.NGL,
		"upstream_url": s.cfg.Upstream,
	})
}

// handleAdminStart 启动 llama-server 子进程（后台常驻）。
func (s *Server) handleAdminStart(w http.ResponseWriter, r *http.Request) {
	var req adminStartReq
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeJSON(w, map[string]string{"ok": "false", "msg": "请求参数不是有效 JSON"})
			return
		}
	}
	modelPath, ctx, ngl, err := s.validateStart(req)
	if err != nil {
		writeJSON(w, map[string]string{"ok": "false", "msg": err.Error()})
		return
	}
	if _, err := os.Stat(s.cfg.LlamaBin); err != nil {
		writeJSON(w, map[string]string{"ok": "false", "msg": "llama-server 不存在: " + s.cfg.LlamaBin})
		return
	}

	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		s.mu.Unlock()
		writeJSON(w, map[string]string{"ok": "false", "msg": "已在运行，请先停止"})
		return
	}
	s.mu.Unlock()
	if s.upstreamReady() {
		writeJSON(w, map[string]string{"ok": "false", "msg": "8080 已被外部 llama-server 占用，请先停止它"})
		return
	}
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		s.mu.Unlock()
		writeJSON(w, map[string]string{"ok": "false", "msg": "已在运行，请先停止"})
		return
	}

	args := []string{
		"-m", modelPath,
		"-c", ctx,
		"-ngl", ngl,
		"--jinja",
		"--host", "127.0.0.1",
		"--port", "8080",
		"--temp", "1.0", "--top-p", "0.95", "--top-k", "64",
	}
	cmd := exec.Command(s.cfg.LlamaBin, args...)
	cmd.Dir = filepath.Dir(s.cfg.LlamaBin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, map[string]string{"ok": "false", "msg": "获取 stdout 失败: " + err.Error()})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, map[string]string{"ok": "false", "msg": "获取 stderr 失败: " + err.Error()})
		return
	}
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		writeJSON(w, map[string]string{"ok": "false", "msg": "启动失败: " + err.Error()})
		return
	}
	s.cmd = cmd
	s.cfg.ModelPath = modelPath
	s.cfg.Ctx = ctx
	s.cfg.NGL = ngl
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd = nil
		}
		s.mu.Unlock()
		if err != nil {
			s.appendLog(fmt.Sprintf("[%s] llama-server 已退出: %v", time.Now().Format("15:04:05"), err))
		} else {
			s.appendLog(fmt.Sprintf("[%s] llama-server 已退出", time.Now().Format("15:04:05")))
		}
	}()

	s.appendLog(fmt.Sprintf("[%s] 启动 llama-server: %s %s",
		time.Now().Format("15:04:05"), s.cfg.LlamaBin, strings.Join(args, " ")))
	go pumpLog(stdout, s)
	go pumpLog(stderr, s)
	writeJSON(w, map[string]string{"ok": "true", "msg": "已启动，模型加载中（请稍候查看日志与状态）"})
}

func pumpLog(rc io.ReadCloser, s *Server) {
	defer rc.Close()
	buf := make([]byte, 4096)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			text := strings.TrimRight(string(buf[:n]), "\r\n")
			for _, l := range strings.Split(text, "\n") {
				if l != "" {
					s.appendLog(l)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// handleAdminStop 停止 llama-server 子进程。
func (s *Server) stopManagedModel() (bool, error) {
	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil {
		s.mu.Unlock()
		return false, nil
	}
	s.mu.Unlock()

	if err := cmd.Process.Kill(); err != nil {
		return false, fmt.Errorf("停止失败: %w", err)
	}
	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
	}
	s.mu.Unlock()
	s.appendLog(fmt.Sprintf("[%s] 已发送停止信号", time.Now().Format("15:04:05")))
	if !s.waitUpstreamDown(10 * time.Second) {
		return true, fmt.Errorf("模型进程已结束，但 8080 仍未释放")
	}
	return true, nil
}

func (s *Server) handleAdminStop(w http.ResponseWriter, r *http.Request) {
	stopped, err := s.stopManagedModel()
	if err != nil {
		writeJSON(w, map[string]string{"ok": "false", "msg": err.Error()})
		return
	}
	if !stopped {
		writeJSON(w, map[string]string{"ok": "false", "msg": "未在运行"})
		return
	}
	writeJSON(w, map[string]string{"ok": "true", "msg": "已停止"})
}

// handleAdminRestart 先停后启。
func (s *Server) handleAdminRestart(w http.ResponseWriter, r *http.Request) {
	if _, err := s.stopManagedModel(); err != nil {
		writeJSON(w, map[string]string{"ok": "false", "msg": err.Error()})
		return
	}
	s.handleAdminStart(w, r)
}

// handleAdminLog 返回日志环形缓冲。
func (s *Server) handleAdminLog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"lines": s.logBuf})
}

// handleAdminGPU 查询 nvidia-smi（无 GPU 时返回错误，前端据此提示）。
func (s *Server) handleAdminGPU(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,memory.used,memory.total,utilization.gpu",
		"--format=csv,noheader").Output()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"lines": strings.Split(strings.TrimSpace(string(out)), "\n"),
	})
}

var _ = log.Printf
