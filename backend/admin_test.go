package main

import (
	"os"
	"path/filepath"
	"testing"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	model := filepath.Join(dir, "gemma.gguf")
	if err := os.WriteFile(model, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewServer(&Config{
		ModelDir:  dir,
		ModelPath: model,
		Ctx:       "16384",
		NGL:       "999",
		LlamaBin:  filepath.Join(dir, "llama-server.exe"),
	}), model
}

func TestValidateStartAcceptsModelAndParameters(t *testing.T) {
	s, model := testServer(t)
	got, ctx, ngl, err := s.validateStart(adminStartReq{ModelPath: model, Ctx: "8192", NGL: "42"})
	if err != nil {
		t.Fatalf("validateStart returned error: %v", err)
	}
	if got != model || ctx != "8192" || ngl != "42" {
		t.Fatalf("unexpected values: %q %q %q", got, ctx, ngl)
	}
}

func TestValidateStartRejectsOutsideModelDirectory(t *testing.T) {
	s, _ := testServer(t)
	_, _, _, err := s.validateStart(adminStartReq{ModelPath: filepath.Join(t.TempDir(), "outside.gguf")})
	if err == nil {
		t.Fatal("expected model directory validation error")
	}
}

func TestValidateStartRejectsInvalidContext(t *testing.T) {
	s, model := testServer(t)
	_, _, _, err := s.validateStart(adminStartReq{ModelPath: model, Ctx: "bad"})
	if err == nil {
		t.Fatal("expected context validation error")
	}
}
