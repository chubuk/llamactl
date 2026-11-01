package main

import (
	"fmt"
	"os"
	"strings"
)

// uiLang: "en" | "ru". Английский — исходные строки; ruCatalog даёт перевод.
var uiLang = "en"

// tr возвращает строку на текущем языке; en — исходный текст и формат одновременно.
func tr(en string, args ...any) string {
	format := en
	if uiLang == "ru" {
		if ru, ok := ruCatalog[en]; ok {
			format = ru
		}
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// detectLocale: "ru" при русской локали, иначе "en".
func detectLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "ru") {
				return "ru"
			}
			return "en"
		}
	}
	return "en"
}

// normalizeLang принимает en/ru (с региональным суффиксом вида ru_RU.UTF-8).
func normalizeLang(v string) (string, bool) {
	switch {
	case strings.HasPrefix(strings.ToLower(v), "ru"):
		return "ru", true
	case strings.HasPrefix(strings.ToLower(v), "en"):
		return "en", true
	}
	return "", false
}

func setLang(v string) {
	uiLang = v
}
