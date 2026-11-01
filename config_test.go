package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.ini")
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("LLAMA_CONFIG", path)
	return path
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"LLAMA_MODELS_DIR", "LLAMA_DISABLED_DIR", "LLAMA_PRESETS", "LLAMA_PORT",
		"LLAMA_FIT_PARAMS", "LLAMA_SERVICE", "VISUAL", "EDITOR",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveSettingsDefaults(t *testing.T) {
	clearConfigEnv(t)
	writeConfig(t, "") // файла нет
	cfg, src, _, err := resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.port != 8033 || cfg.service != "llama-server" {
		t.Errorf("defaults: port=%d service=%q", cfg.port, cfg.service)
	}
	if cfg.presets != filepath.Join(cfg.modelsDir, "presets.ini") {
		t.Errorf("presets default should follow models-dir: %q", cfg.presets)
	}
	if src["port"].source != "default" || src["editor"].source != "default" {
		t.Errorf("sources: %v", src)
	}
}

func TestResolveSettingsFileBeatsDefault(t *testing.T) {
	clearConfigEnv(t)
	writeConfig(t, "# комментарий\nport = 9000\nmodels-dir = /tmp/models\n\nservice = my-llama\n")
	cfg, src, _, err := resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.port != 9000 || cfg.modelsDir != "/tmp/models" || cfg.service != "my-llama" {
		t.Errorf("file values not applied: %+v", cfg)
	}
	if cfg.presets != "/tmp/models/presets.ini" {
		t.Errorf("presets should follow file models-dir: %q", cfg.presets)
	}
	if src["port"].source != "file" {
		t.Errorf("port source = %q, want file", src["port"].source)
	}
}

func TestResolveSettingsEnvBeatsFile(t *testing.T) {
	clearConfigEnv(t)
	writeConfig(t, "port = 9000\n")
	t.Setenv("LLAMA_PORT", "7777")
	cfg, src, _, err := resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.port != 7777 {
		t.Errorf("port = %d, want env 7777", cfg.port)
	}
	if src["port"].source != "env LLAMA_PORT" {
		t.Errorf("port source = %q, want env LLAMA_PORT", src["port"].source)
	}
}

func TestResolveSettingsEditorPrecedence(t *testing.T) {
	clearConfigEnv(t)
	writeConfig(t, "editor = vim\n")
	cfg, src, _, err := resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.editor != "vim" || src["editor"].source != "file" {
		t.Errorf("editor = %q (%s), want vim (file)", cfg.editor, src["editor"].source)
	}
	t.Setenv("EDITOR", "nano2")
	cfg, _, _, err = resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.editor != "nano2" {
		t.Errorf("editor = %q, want env EDITOR to win", cfg.editor)
	}
}

func TestResolveSettingsErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bad port", "port = abc\n"},
		{"port out of range", "port = 70000\n"},
		{"unknown key", "modles-dir = /tmp\n"},
		{"not kv", "просто строка\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			writeConfig(t, tt.body)
			if _, _, _, err := resolveSettings(); err == nil {
				t.Errorf("resolveSettings(%q) err = nil, want error", tt.body)
			}
		})
	}
}
