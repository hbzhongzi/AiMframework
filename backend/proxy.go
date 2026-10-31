package main

import (
	"io"
	"log"
	"net/http"
)

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// proxy 将请求转发到 llama-server 的 upstreamPath，并原样回流响应。
// 对 SSE 流式（chat/completions）也能逐块 flush，保证前端实时逐字显示。
func (s *Server) proxy(w http.ResponseWriter, r *http.Request, upstreamPath string) {
	target := s.cfg.Upstream + upstreamPath

	req, err := http.NewRequest(r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	copyHeader(req.Header, r.Header)
	req.Header.Set("Authorization", "Bearer "+s.cfg.UpstreamKey)
	req.Header.Del("Host")

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[proxy] upstream error: %v", err)
		http.Error(w, "upstream unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 8192)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			break
		}
	}
}
