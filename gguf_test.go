package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeTestGGUF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.gguf")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := func(v any) {
		if err := binary.Write(f, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	wstr := func(s string) {
		w(uint64(len(s)))
		if _, err := f.WriteString(s); err != nil {
			t.Fatal(err)
		}
	}

	f.WriteString("GGUF")
	w(uint32(3)) // version
	w(uint64(0)) // tensor_count
	w(uint64(4)) // kv_count

	wstr("general.architecture")
	w(uint32(8)) // string
	wstr("qwen2")

	wstr("qwen2.block_count")
	w(uint32(4)) // uint32
	w(uint32(28))

	wstr("qwen2.context_length")
	w(uint32(10)) // uint64
	w(uint64(32768))

	wstr("tokenizer.ggml.tokens")
	w(uint32(9)) // array
	w(uint32(8)) // elem: string
	w(uint64(2)) // count
	wstr("a")
	wstr("b")

	return path
}

func TestGgufMeta(t *testing.T) {
	path := writeTestGGUF(t)
	pairs, err := ggufMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"general.architecture": "qwen2",
		"qwen2.block_count":    "28",
		"qwen2.context_length": "32768",
	}
	if len(pairs) != len(want) {
		t.Fatalf("ggufMeta() = %v, want %d pairs", pairs, len(want))
	}
	for _, p := range pairs {
		if want[p.key] != p.value {
			t.Errorf("pair %q = %q, want %q", p.key, p.value, want[p.key])
		}
	}
}

func TestGgufMetaRejectsNonGGUF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.gguf")
	if err := os.WriteFile(path, []byte("XXXX\x03\x00\x00\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ggufMeta(path); err == nil {
		t.Error("ggufMeta(non-gguf) err = nil, want error")
	}
}
