//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	llamaDir   = `D:\llama-cpp`
	modelPath  = `D:\models\gemma-4-12b-Q4_K_M.gguf`
	adminURL   = "http://127.0.0.1:8081/admin"
	chatURL    = "http://127.0.0.1:8081/chat"
)

// defaultPage is overridden for the chat launcher build with -ldflags -X.
var defaultPage = "admin"

func main() {
	root, err := projectRoot()
	if err != nil {
		showError(err)
		return
	}

	llamaExe := filepath.Join(llamaDir, "llama-server.exe")
	backendExe := filepath.Join(root, "dist", "gemma-backend.exe")
	backendWorkDir := filepath.Join(root, "backend")
	for _, p := range []string{llamaExe, modelPath, backendExe} {
		if _, err := os.Stat(p); err != nil {
			showError(fmt.Errorf("missing required file: %s", p))
			return
		}
	}

	if !isHealthy("http://127.0.0.1:8081/admin/status") {
		if err := startHidden(backendExe, backendWorkDir); err != nil {
			showError(fmt.Errorf("failed to start Go backend: %w", err))
			return
		}
	}

	if !waitFor("http://127.0.0.1:8081/admin/status", 30*time.Second) {
		showError(fmt.Errorf("the Go control service did not become ready"))
		return
	}

	// Keep model lifecycle under backend/admin.go so status, stop, restart, and logs agree.
	if !isHealthy("http://127.0.0.1:8080/health") {
		payload, _ := json.Marshal(map[string]string{
			"model_path": modelPath,
			"ctx":        "16384",
			"ngl":        "999",
		})
		resp, err := http.Post("http://127.0.0.1:8081/admin/start", "application/json", bytes.NewReader(payload))
		if err != nil {
			showError(fmt.Errorf("failed to request model start: %w", err))
			return
		}
		resp.Body.Close()
	}

	if !waitFor("http://127.0.0.1:8080/health", 10*time.Minute) {
		showError(fmt.Errorf("the local model service did not become ready; check llama-server logs"))
		return
	}

	page := adminURL
	if defaultPage == "chat" {
		page = chatURL
	}
	if err := openAppWindow(page); err != nil {
		showError(err)
	}
}

func projectRoot() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if strings.EqualFold(filepath.Base(dir), "dist") {
		return filepath.Dir(dir), nil
	}
	return dir, nil
}

func startHidden(exe, dir string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func isHealthy(url string) bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func waitFor(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHealthy(url) {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func openAppWindow(url string) error {
	for _, p := range edgeCandidates() {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, "--app="+url, "--start-maximized")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return cmd.Start()
		}
	}
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func edgeCandidates() []string {
	paths := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) != "" {
			result = append(result, p)
		}
	}
	return result
}

func showError(err error) {
	message, _ := syscall.UTF16PtrFromString(err.Error())
	title, _ := syscall.UTF16PtrFromString("Gemma Desktop")
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
