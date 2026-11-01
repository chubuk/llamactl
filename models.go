package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ctxRe = regexp.MustCompile(`^[0-9]+$`)

func stdinReader() *bufio.Reader { return bufio.NewReader(os.Stdin) }

func promptLine(r *bufio.Reader, format string, args ...any) string {
	fmt.Printf(format, args...)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

// confirm: def=true — по умолчанию "да" ([Y/n]).
func confirm(r *bufio.Reader, question string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	ans := promptLine(r, "%s %s: ", question, hint)
	switch strings.ToLower(ans) {
	case "y", "yes", "д", "да":
		return true
	case "n", "no", "н", "нет":
		return false
	case "":
		return def
	}
	return false
}

func listGguf(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gguf") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func modelName(file string) string {
	return strings.TrimSuffix(filepath.Base(file), ".gguf")
}

func cmdList(cfg config) error {
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	fmt.Printf(tr("=== Active models (%s) ===")+"\n", cfg.modelsDir)
	active, err := listGguf(cfg.modelsDir)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		fmt.Println(tr("  (empty)"))
	}
	for _, f := range active {
		name := modelName(f)
		st, _ := os.Stat(filepath.Join(cfg.modelsDir, f))
		size := int64(0)
		if st != nil {
			size = st.Size()
		}
		mark := "✗"
		if ok, _ := presetExists(cfg.presets, name); ok {
			mark = "✓"
		}
		fmt.Printf("  %-60s %10s  preset:%s\n", name, humanBytes(size), mark)
	}

	fmt.Println()
	fmt.Printf(tr("=== Disabled models (%s) ===")+"\n", cfg.disabledDir)
	disabled, err := listGguf(cfg.disabledDir)
	if err != nil {
		return err
	}
	if len(disabled) == 0 {
		fmt.Println(tr("  (empty)"))
	}
	for _, f := range disabled {
		fmt.Printf("  %s\n", modelName(f))
	}

	fmt.Println()
	fmt.Println(tr("=== Loaded in API ==="))
	ids, err := listLoadedModels(cfg)
	if err != nil {
		fmt.Println(tr("  (server not running)"))
		return nil
	}
	if len(ids) == 0 {
		fmt.Println("  (пусто)")
	}
	for _, id := range ids {
		fmt.Printf("  %s\n", id)
	}
	return nil
}

func listLoadedModels(cfg config) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.baseURL()+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("статус %s", resp.Status)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("не JSON: %w", err)
	}
	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func requireName(args []string, usageLine string) string {
	if len(args) < 1 || args[0] == "" {
		usage(tr("specify model name (without .gguf)\nExample: %s", usageLine))
	}
	return args[0]
}

func restartService(cfg config) error {
	fmt.Println(tr("Restarting server..."))
	unit := activeUnit(cfg)
	if out, err := runSystemctl(cfg, false, "restart", unit); err != nil {
		return fmt.Errorf(tr("restarting %s: %w: %s"), unit, err, strings.TrimSpace(out))
	}
	if err := waitReady(cfg, 30*time.Second); err != nil {
		return err
	}
	fmt.Println(tr("Done"))
	return nil
}

// offerPreset: предложить tune; при отказе — шаблонный пресет с запросом ctx (как в bash-версии).
func offerPreset(cfg config, name string, forced map[string]string) error {
	r := stdinReader()
	if confirm(r, tr("Configure preset for [%s] via llama tune?", name), true) {
		var args []string
		args = append(args, name)
		for _, k := range tuneOrderFromMap(forced) {
			args = append(args, "--"+k, forced[k])
		}
		return cmdTune(cfg, args)
	}
	exists, err := presetExists(cfg.presets, name)
	if err != nil {
		return err
	}
	if exists {
		fmt.Println(tr("preset already exists: [%s]", name))
		return nil
	}
	ctxSize := "16384"
	if v, ok := forced["ctx-size"]; ok {
		ctxSize = v
	} else {
		for {
			ans := promptLine(r, tr("Enter ctx-size for [%s] (Enter = %s): "), name, ctxSize)
			if ans == "" {
				break
			}
			if ctxRe.MatchString(ans) {
				ctxSize = ans
				break
			}
			fmt.Println(" ", tr("expected a positive number"))
		}
	}
	return upsertSection(cfg.presets, name, []kv{
		{"ctx-size", ctxSize},
		{"n-gpu-layers", "99"},
		{"cache-type-k", "q4_0"},
		{"cache-type-v", "q4_0"},
		{"flash-attn", "on"},
		{"parallel", "1"},
		{"threads", fmt.Sprintf("%d", max(1, nproc()-2))},
	})
}

func tuneOrderFromMap(m map[string]string) []string {
	order := make([]string, 0, len(m))
	for k := range m {
		order = append(order, k)
	}
	sort.Strings(order)
	return order
}

func parseCtxFlag(args []string) (rest []string, forced map[string]string) {
	forced = make(map[string]string)
	for i := 0; i < len(args); i++ {
		if args[i] == "--ctx" && i+1 < len(args) {
			forced["ctx-size"] = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--ctx=") {
			forced["ctx-size"] = strings.TrimPrefix(args[i], "--ctx=")
			continue
		}
		rest = append(rest, args[i])
	}
	return rest, forced
}

func cmdAdd(cfg config, args []string) error {
	args, forced := parseCtxFlag(args)
	if len(args) < 1 || args[0] == "" {
		usage(tr("specify HuggingFace repository\nExample: llama add bartowski/Qwen2.5-Coder-7B-Instruct-GGUF Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf"))
	}
	if _, err := exec.LookPath("hf"); err != nil {
		return errors.New(tr("command 'hf' not found, install: pip3 install --user huggingface_hub[cli]"))
	}
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	repo := args[0]
	file := ""
	if len(args) > 1 {
		file = args[1]
	}

	before, err := listGguf(cfg.modelsDir)
	if err != nil {
		return err
	}

	fmt.Println(tr("=== Downloading model ==="))
	fmt.Println(tr("Repository: %s", repo))
	if file != "" {
		fmt.Println(tr("File: %s", file))
	} else {
		fmt.Println(tr("(no file specified — the whole repository will be downloaded)"))
	}

	hfArgs := []string{"download", repo}
	if file != "" {
		hfArgs = append(hfArgs, file)
	}
	hfArgs = append(hfArgs, "--local-dir", cfg.modelsDir)
	hf := exec.Command("hf", hfArgs...)
	hf.Stdin = os.Stdin
	hf.Stdout = os.Stdout
	hf.Stderr = os.Stderr
	if err := hf.Run(); err != nil {
		return fmt.Errorf(tr("download: %w"), err)
	}

	after, err := listGguf(cfg.modelsDir)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(before))
	for _, f := range before {
		seen[f] = true
	}
	var fresh []string
	for _, f := range after {
		if !seen[f] {
			fresh = append(fresh, f)
		}
	}

	fmt.Println()
	fmt.Printf(tr("=== New .gguf in %s ===")+"\n", cfg.modelsDir)
	if len(fresh) == 0 {
		fmt.Println(tr("  (none found — the file may already exist)"))
	}
	for _, f := range fresh {
		fmt.Println(" ", modelName(f))
	}

	for _, f := range fresh {
		fmt.Println()
		if err := offerPreset(cfg, modelName(f), forced); err != nil {
			return err
		}
	}
	fmt.Println()
	return restartService(cfg)
}

func cmdRemove(cfg config, args []string) error {
	name := requireName(args, "llama remove Qwen2.5-Coder-7B-Instruct-Q4_K_M")
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	file := filepath.Join(cfg.modelsDir, name+".gguf")
	st, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf(tr("model not found: %s (see llama list)"), file)
	}
	r := stdinReader()
	fmt.Println(tr("Model to delete: %s (%s)", name, humanBytes(st.Size())))
	if !confirm(r, tr("Confirm deletion?"), false) {
		fmt.Println(tr("Cancelled"))
		return nil
	}
	if exists, err := presetExists(cfg.presets, name); err != nil {
		return err
	} else if exists {
		if _, err := removeSection(cfg.presets, name); err != nil {
			return err
		}
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	fmt.Println(tr("File deleted: %s", file))
	return restartService(cfg)
}

func cmdDisable(cfg config, args []string) error {
	name := requireName(args, "llama disable Qwen2.5-Coder-7B-Instruct-Q4_K_M")
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	src := filepath.Join(cfg.modelsDir, name+".gguf")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf(tr("model not found: %s (see llama list)"), name)
	}
	if err := os.Rename(src, filepath.Join(cfg.disabledDir, name+".gguf")); err != nil {
		return err
	}
	fmt.Println(tr("Model moved to %s", cfg.disabledDir))
	if exists, err := presetExists(cfg.presets, name); err != nil {
		return err
	} else if exists {
		if _, err := removeSection(cfg.presets, name); err != nil {
			return err
		}
	}
	if err := restartService(cfg); err != nil {
		return err
	}
	fmt.Println(tr("Model disabled"))
	return nil
}

func cmdEnable(cfg config, args []string) error {
	args, forced := parseCtxFlag(args)
	name := requireName(args, "llama enable Qwen2.5-Coder-7B-Instruct-Q4_K_M")
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	src := filepath.Join(cfg.disabledDir, name+".gguf")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf(tr("model not found in %s: %s"), cfg.disabledDir, name)
	}
	if err := os.Rename(src, filepath.Join(cfg.modelsDir, name+".gguf")); err != nil {
		return err
	}
	fmt.Println(tr("Model returned to %s", cfg.modelsDir))
	if err := offerPreset(cfg, name, forced); err != nil {
		return err
	}
	if err := restartService(cfg); err != nil {
		return err
	}
	fmt.Println(tr("Done"))
	return nil
}
