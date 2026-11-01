package main

import "testing"

func TestTrLanguages(t *testing.T) {
	defer setLang("en")

	setLang("en")
	if got := tr("Stopped"); got != "Stopped" {
		t.Errorf("tr(en) = %q, want Stopped", got)
	}
	setLang("ru")
	if got := tr("Stopped"); got != "Остановлен" {
		t.Errorf("tr(ru) = %q, want Остановлен", got)
	}
	if got := tr("Model moved to %s", "/tmp"); got != "✓ Модель перемещена в /tmp" {
		t.Errorf("tr(ru, args) = %q", got)
	}
	// неизвестный ключ — фолбэк на английский
	if got := tr("no such key %d", 7); got != "no such key 7" {
		t.Errorf("tr fallback = %q", got)
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"ru", "ru", true},
		{"ru_RU.UTF-8", "ru", true},
		{"en_US.UTF-8", "en", true},
		{"en", "en", true},
		{"de", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeLang(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("normalizeLang(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDetectLocale(t *testing.T) {
	defer setLang("en")

	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ru_RU.UTF-8")
	if got := detectLocale(); got != "ru" {
		t.Errorf("detectLocale(ru_RU) = %q, want ru", got)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if got := detectLocale(); got != "en" {
		t.Errorf("detectLocale(en_US) = %q, want en", got)
	}
	t.Setenv("LANG", "")
	if got := detectLocale(); got != "en" {
		t.Errorf("detectLocale(нет локали) = %q, want en", got)
	}
}

func TestLangFromConfigAndEnv(t *testing.T) {
	defer setLang("en")

	// lang из файла
	clearConfigEnv(t)
	t.Setenv("LLAMA_LANG", "")
	writeConfig(t, "lang = ru\n")
	cfg, src, _, err := resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	_ = cfg
	if src["lang"].value != "ru" || src["lang"].source != "file" {
		t.Errorf("lang = %v, want ru/file", src["lang"])
	}
	if uiLang != "ru" {
		t.Errorf("uiLang = %q, want ru", uiLang)
	}

	// env перебивает файл
	t.Setenv("LLAMA_LANG", "en_US.UTF-8")
	_, src, _, err = resolveSettings()
	if err != nil {
		t.Fatal(err)
	}
	if src["lang"].value != "en" || src["lang"].source != "env LLAMA_LANG" {
		t.Errorf("lang = %v, want en/env LLAMA_LANG", src["lang"])
	}

	// недопустимое значение — ошибка
	t.Setenv("LLAMA_LANG", "de")
	if _, _, _, err := resolveSettings(); err == nil {
		t.Error("resolveSettings(lang=de) err = nil, want error")
	}
}
