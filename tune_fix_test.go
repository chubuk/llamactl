package main

import (
	"strings"
	"testing"
)

func TestSectionValue(t *testing.T) {
	sec := "[m]\nctx-size = 32768\nn-cpu-moe = 28\njinja = true\n"
	if v, ok := sectionValue(sec, "n-cpu-moe"); !ok || v != "28" {
		t.Errorf("sectionValue(n-cpu-moe) = (%q, %v), want (28, true)", v, ok)
	}
	if _, ok := sectionValue(sec, "missing"); ok {
		t.Error("sectionValue(missing) ok = true, want false")
	}
}

func TestInheritPlacement(t *testing.T) {
	sec := "[m]\nn-cpu-moe = 28\nctx-size = 4096\n"

	// наследуется из секции
	forced := map[string]string{}
	var order []string
	inheritPlacement(sec, forced, &order)
	if forced["n-cpu-moe"] != "28" || len(order) != 1 {
		t.Errorf("inherit: forced=%v order=%v", forced, order)
	}

	// явное значение пользователя не перетирается
	forced = map[string]string{"n-cpu-moe": "12"}
	order = nil
	inheritPlacement(sec, forced, &order)
	if forced["n-cpu-moe"] != "12" || len(order) != 0 {
		t.Errorf("explicit wins: forced=%v order=%v", forced, order)
	}

	// override-tensor не наследуется (только простые флаги)
	secOT := "[m]\noverride-tensor = a=CPU\n"
	forced = map[string]string{}
	order = nil
	inheritPlacement(secOT, forced, &order)
	if _, ok := forced["override-tensor"]; ok {
		t.Error("override-tensor must not be inherited")
	}
}

func TestUpsertTrimsTrailingBlanks(t *testing.T) {
	path := writePresets(t, "[m]\nctx-size = 4096\nchat-template-kwargs = {}\n\n\n\n")
	if err := upsertSection(path, "m", []kv{{"ctx-size", "213248"}}); err != nil {
		t.Fatal(err)
	}
	got := readPresets(t, path)
	// новые ключи не должны дописываться после пустоты; хвостовые пустые строки убраны
	if strings.Contains(got, "213248\n\n") {
		t.Errorf("gap after updated key:\n%q", got)
	}
	if !strings.Contains(got, "ctx-size = 213248\nchat-template-kwargs = {}") {
		t.Errorf("keys not compact:\n%s", got)
	}
	// последний непустой контент — секция (без хвостовых пустых строк)
	if strings.HasSuffix(strings.ReplaceAll(got, "\n", ""), "chat-template-kwargs={}") && !strings.HasSuffix(got, "\n") {
		t.Errorf("file should end with newline:\n%q", got)
	}
}

func TestDropSectionKey(t *testing.T) {
	path := writePresets(t, "[m]\nctx-size = 1\noverride-tensor = blk\\.1=CPU,blk\\.2=CPU\nthreads = 10\n")
	if err := dropSectionKey(path, "m", "override-tensor"); err != nil {
		t.Fatal(err)
	}
	got := readPresets(t, path)
	if strings.Contains(got, "override-tensor") {
		t.Errorf("override-tensor survived:\n%s", got)
	}
	for _, want := range []string{"ctx-size = 1", "threads = 10"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q lost:\n%s", want, got)
		}
	}
}
