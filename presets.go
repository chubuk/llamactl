package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

type kv struct {
	key   string
	value string
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

func writeLines(path string, lines []string) error {
	body := strings.Join(lines, "\n")
	if len(lines) > 0 {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func sectionHeader(name string) string { return "[" + name + "]" }

func isSectionLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")
}

// findSection возвращает границы секции [start, end): start — строка заголовка.
func findSection(lines []string, name string) (start, end int) {
	header := sectionHeader(name)
	for i, ln := range lines {
		if strings.TrimSpace(ln) != header {
			continue
		}
		start = i
		end = len(lines)
		for j := i + 1; j < len(lines); j++ {
			if isSectionLine(lines[j]) {
				end = j
				break
			}
		}
		return start, end
	}
	return -1, -1
}

func parseKVLine(line string) (kv, bool) {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") || strings.HasPrefix(t, "[") {
		return kv{}, false
	}
	k, v, found := strings.Cut(t, "=")
	if !found {
		return kv{}, false
	}
	return kv{key: strings.TrimSpace(k), value: strings.TrimSpace(v)}, true
}

func presetExists(path, name string) (bool, error) {
	lines, err := readLines(path)
	if err != nil {
		return false, err
	}
	start, _ := findSection(lines, name)
	return start >= 0, nil
}

func presetShow(path, name string) (string, error) {
	lines, err := readLines(path)
	if err != nil {
		return "", err
	}
	start, end := findSection(lines, name)
	if start < 0 {
		return "", nil
	}
	return strings.Join(lines[start:end], "\n"), nil
}

func formatKV(k kv) string { return k.key + " = " + k.value }

// upsertSection создает/обновляет секцию [name]; существующие ключи вне newKeys сохраняются.
func upsertSection(path, name string, newKeys []kv) error {
	return withPresetsLock(path, func() error {
		if err := backupPresets(path); err != nil {
			return err
		}
		lines, err := readLines(path)
		if err != nil {
			return err
		}
		newMap := make(map[string]string, len(newKeys))
		for _, k := range newKeys {
			newMap[k.key] = k.value
		}
		start, end := findSection(lines, name)
		var out []string
		if start >= 0 {
			merged := make([]string, 0, len(newKeys))
			seen := make(map[string]bool, len(newKeys))
			for _, ln := range lines[start+1 : end] {
				if pk, ok := parseKVLine(ln); ok {
					if nv, hit := newMap[pk.key]; hit {
						merged = append(merged, formatKV(kv{pk.key, nv}))
						seen[pk.key] = true
						continue
					}
				}
				merged = append(merged, ln)
			}
			// Хвостовые пустые строки секции (в т.ч. до конца файла) убираем,
			// чтобы новые ключи не дописывались после «воздуха».
			for len(merged) > 0 && strings.TrimSpace(merged[len(merged)-1]) == "" {
				merged = merged[:len(merged)-1]
			}
			for _, k := range newKeys {
				if !seen[k.key] {
					merged = append(merged, formatKV(k))
				}
			}
			out = append(out, lines[:start]...)
			out = append(out, sectionHeader(name))
			out = append(out, merged...)
			out = append(out, lines[end:]...)
		} else {
			out = append(out, lines...)
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, sectionHeader(name))
			for _, k := range newKeys {
				out = append(out, formatKV(k))
			}
		}
		return writeLines(path, out)
	})
}

// sectionValue возвращает значение ключа из текста секции пресета.
func sectionValue(section, key string) (string, bool) {
	for _, ln := range strings.Split(section, "\n") {
		if p, ok := parseKVLine(ln); ok && p.key == key {
			return p.value, true
		}
	}
	return "", false
}

// dropSectionKey удаляет ключ из секции (для чистки устаревших ключей размещения).
func dropSectionKey(path, name, key string) error {
	return withPresetsLock(path, func() error {
		lines, err := readLines(path)
		if err != nil {
			return err
		}
		start, end := findSection(lines, name)
		if start < 0 {
			return nil
		}
		if err := backupPresets(path); err != nil {
			return err
		}
		kept := make([]string, 0, end-start)
		for _, ln := range lines[start+1 : end] {
			if p, ok := parseKVLine(ln); ok && p.key == key {
				continue
			}
			kept = append(kept, ln)
		}
		for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
			kept = kept[:len(kept)-1]
		}
		out := make([]string, 0, len(lines)-1)
		out = append(out, lines[:start]...)
		out = append(out, sectionHeader(name))
		out = append(out, kept...)
		out = append(out, lines[end:]...)
		return writeLines(path, out)
	})
}

func removeSection(path, name string) (bool, error) {
	removed := false
	err := withPresetsLock(path, func() error {
		lines, err := readLines(path)
		if err != nil {
			return err
		}
		start, end := findSection(lines, name)
		if start < 0 {
			return nil
		}
		if err := backupPresets(path); err != nil {
			return err
		}
		removed = true
		return writeLines(path, append(lines[:start:start], lines[end:]...))
	})
	return removed, err
}

func backupPresets(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(path+".bak", data, 0o644)
}

func withPresetsLock(path string, fn func() error) error {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf(tr("lock %s.lock: %w"), path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

func ensureDirs(cfg config) error {
	for _, d := range []string{cfg.modelsDir, cfg.disabledDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(cfg.presets); os.IsNotExist(err) {
		return os.WriteFile(cfg.presets, nil, 0o644)
	}
	return nil
}

func cmdPreset(cfg config, args []string) error {
	if len(args) < 1 || args[0] == "" {
		usage(tr("specify model name: llama preset <name>"))
	}
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	name := args[0]
	text, err := presetShow(cfg.presets, name)
	if err != nil {
		return err
	}
	if text == "" {
		return fmt.Errorf(tr("preset not found for: %s"), name)
	}
	fmt.Println(text)
	return nil
}

func cmdPresets(cfg config) error {
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	return openInEditor(cfg.editor, cfg.presets)
}
