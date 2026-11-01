package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePresets(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presets.ini")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readPresets(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPresetExists(t *testing.T) {
	path := writePresets(t, "[Qwen2.5-X]\nctx-size = 1\n\n[other]\nk = v\n")
	tests := []struct {
		name string
		want bool
	}{
		{"Qwen2.5-X", true},
		{"Qwen2X5-X", false}, // точки — не «любой символ»
		{"Qwen2.5-X-extra", false},
		{"missing", false},
	}
	for _, tt := range tests {
		got, err := presetExists(path, tt.name)
		if err != nil {
			t.Fatalf("presetExists(%q): %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("presetExists(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUpsertSectionCreatesAndMerges(t *testing.T) {
	path := writePresets(t, "[keep]\nctx-size = 7\n")

	if err := upsertSection(path, "Qwen2.5-X", []kv{{"ctx-size", "16384"}, {"threads", "10"}}); err != nil {
		t.Fatal(err)
	}
	got := readPresets(t, path)
	for _, want := range []string{"[keep]", "ctx-size = 7", "[Qwen2.5-X]", "ctx-size = 16384", "threads = 10"} {
		if !strings.Contains(got, want) {
			t.Errorf("after create: %q not in\n%s", want, got)
		}
	}

	// merge: обновить один ключ, сохранить чужой ключ и комментарий.
	if err := upsertSection(path, "Qwen2.5-X", []kv{{"ctx-size", "32768"}, {"flash-attn", "on"}}); err != nil {
		t.Fatal(err)
	}
	got = readPresets(t, path)
	for _, want := range []string{"ctx-size = 32768", "threads = 10", "flash-attn = on"} {
		if !strings.Contains(got, want) {
			t.Errorf("after merge: %q not in\n%s", want, got)
		}
	}
	if strings.Contains(got, "ctx-size = 16384") {
		t.Errorf("old ctx-size value survived:\n%s", got)
	}
}

func TestUpsertPreservesCustomKeys(t *testing.T) {
	path := writePresets(t, "[m1]\nctx-size = 1\nchat-template-kwargs = {\"enable_thinking\": false}\n")
	if err := upsertSection(path, "m1", []kv{{"ctx-size", "2"}}); err != nil {
		t.Fatal(err)
	}
	got := readPresets(t, path)
	if !strings.Contains(got, `chat-template-kwargs = {"enable_thinking": false}`) {
		t.Errorf("custom key lost:\n%s", got)
	}
	if !strings.Contains(got, "ctx-size = 2") {
		t.Errorf("ctx-size not updated:\n%s", got)
	}
}

func TestRemoveSection(t *testing.T) {
	path := writePresets(t, "[a]\nk = 1\n[b]\nk = 2\n[c]\nk = 3\n")
	removed, err := removeSection(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("removeSection(=b) removed = false, want true")
	}
	got := readPresets(t, path)
	if strings.Contains(got, "[b]") || strings.Contains(got, "k = 2") {
		t.Errorf("section b survived:\n%s", got)
	}
	for _, want := range []string{"[a]", "k = 1", "[c]", "k = 3"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q lost:\n%s", want, got)
		}
	}

	removed, err = removeSection(path, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("removeSection(missing) removed = true, want false")
	}
}

func TestPresetShow(t *testing.T) {
	path := writePresets(t, "[a]\nk = 1\nj = 2\n[b]\nk = 3\n")
	got, err := presetShow(path, "a")
	if err != nil {
		t.Fatal(err)
	}
	want := "[a]\nk = 1\nj = 2"
	if got != want {
		t.Errorf("presetShow(a) = %q, want %q", got, want)
	}
	got, err = presetShow(path, "zzz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("presetShow(zzz) = %q, want empty", got)
	}
}
