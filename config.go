package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Ключи config.ini; неизвестный ключ — ошибка (защита от опечаток).
var configKeys = []string{
	"models-dir", "disabled-dir", "presets", "port", "fit-params", "service", "editor", "lang",
}

// configFile: $LLAMA_CONFIG или $XDG_CONFIG_HOME/llamactl/config.ini.
func configFile() (string, error) {
	if p := os.Getenv("LLAMA_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf(tr("config directory: %w"), err)
	}
	return filepath.Join(dir, "llamactl", "config.ini"), nil
}

func loadConfigFile(path string) (map[string]string, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	valid := make(map[string]bool, len(configKeys))
	for _, k := range configKeys {
		valid[k] = true
	}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		p, ok := parseKVLine(ln)
		if !ok {
			return nil, fmt.Errorf(tr("%s:%d: expected «key = value», got: %s"), path, i+1, t)
		}
		if !valid[p.key] {
			return nil, fmt.Errorf(tr("%s:%d: unknown key «%s» (valid: %s)"), path, i+1, p.key, strings.Join(configKeys, ", "))
		}
		out[p.key] = p.value
	}
	return out, nil
}

type setting struct {
	value  string
	source string // "default" | "file" | "env <ИМЯ>" | "locale"
}

func pick(file map[string]string, fileKey, envKey, def string) setting {
	if v := os.Getenv(envKey); v != "" {
		return setting{v, "env " + envKey}
	}
	if v, ok := file[fileKey]; ok && v != "" {
		return setting{v, "file"}
	}
	return setting{def, "default"}
}

func pickEditor(file map[string]string) setting {
	if v := os.Getenv("VISUAL"); v != "" {
		return setting{v, "env VISUAL"}
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return setting{v, "env EDITOR"}
	}
	if v, ok := file["editor"]; ok && v != "" {
		return setting{v, "file"}
	}
	return setting{"nano", "default"}
}

// pickLang: LLAMA_LANG > lang в файле > автоопределение по локали.
func pickLang(file map[string]string) setting {
	if v := os.Getenv("LLAMA_LANG"); v != "" {
		if n, ok := normalizeLang(v); ok {
			return setting{n, "env LLAMA_LANG"}
		}
		return setting{v, "env LLAMA_LANG"} // ошибка валидируется ниже
	}
	if v, ok := file["lang"]; ok && v != "" {
		if n, ok := normalizeLang(v); ok {
			return setting{n, "file"}
		}
		return setting{v, "file"}
	}
	return setting{detectLocale(), "locale"}
}

// resolveSettings: дефолты < config.ini < env. Возвращает и источник каждого значения.
func resolveSettings() (config, map[string]setting, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, nil, "", fmt.Errorf(tr("home directory: %w"), err)
	}

	path, err := configFile()
	if err != nil {
		return config{}, nil, "", err
	}
	file := map[string]string{}
	if _, statErr := os.Stat(path); statErr == nil {
		file, err = loadConfigFile(path)
		if err != nil {
			return config{}, nil, path, err
		}
	}

	src := make(map[string]setting)
	modelsDir := pick(file, "models-dir", "LLAMA_MODELS_DIR", filepath.Join(home, "models"))
	src["models-dir"] = modelsDir

	disabledDir := pick(file, "disabled-dir", "LLAMA_DISABLED_DIR", filepath.Join(home, "models-disabled"))
	src["disabled-dir"] = disabledDir

	presets := pick(file, "presets", "LLAMA_PRESETS", filepath.Join(modelsDir.value, "presets.ini"))
	src["presets"] = presets

	port := pick(file, "port", "LLAMA_PORT", "8033")
	src["port"] = port

	fitParams := pick(file, "fit-params", "LLAMA_FIT_PARAMS", filepath.Join(home, "llama.cpp", "build", "bin", "llama-fit-params"))
	src["fit-params"] = fitParams

	service := pick(file, "service", "LLAMA_SERVICE", "llama-server")
	src["service"] = service

	editor := pickEditor(file)
	src["editor"] = editor

	lang := pickLang(file)
	src["lang"] = lang
	if l, ok := normalizeLang(lang.value); ok {
		setLang(l)
	} else {
		return config{}, nil, path, fmt.Errorf(tr("lang: invalid value «%s» (valid: en, ru)"), lang.value)
	}

	portNum, err := strconv.Atoi(port.value)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return config{}, nil, path, fmt.Errorf(tr("port: invalid value «%s» (expected 1–65535)"), port.value)
	}

	return config{
		home:        home,
		modelsDir:   modelsDir.value,
		disabledDir: disabledDir.value,
		presets:     presets.value,
		port:        portNum,
		fitParams:   fitParams.value,
		service:     service.value,
		editor:      editor.value,
	}, src, path, nil
}

func cmdConfig() error {
	_, src, path, err := resolveSettings()
	if err != nil {
		return err
	}
	state := tr("not found")
	if _, err := os.Stat(path); err == nil {
		state = tr("found")
	}
	fmt.Println(tr("=== llama settings ==="))
	fmt.Println(tr("Config file: %s (%s)", path, state))
	fmt.Println(tr("Precedence: env > file > default"))
	fmt.Println()
	for _, k := range configKeys {
		s := src[k]
		fmt.Printf("  %-12s = %-50s (%s)\n", k, s.value, s.source)
	}
	return nil
}
