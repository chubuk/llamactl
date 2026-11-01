package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type fitResult struct {
	ctxSize        int
	nGPULayers     int
	tensorSplit    string
	overrideTensor string
}

// splitQuoted режет строку по пробелам, уважая двойные кавычки (кавычки снимаются).
func splitQuoted(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inQuote {
				b.WriteRune(r)
			} else if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func parseFitOutput(s string) (fitResult, error) {
	toks := splitQuoted(strings.TrimSpace(s))
	var r fitResult
	need := func(i int, flag string) (string, error) {
		if i+1 >= len(toks) {
			return "", fmt.Errorf(tr("missing value after %s in fit-params output"), flag)
		}
		return toks[i+1], nil
	}
	for i := 0; i < len(toks); i++ {
		switch toks[i] {
		case "-c", "--ctx-size":
			v, err := need(i, toks[i])
			if err != nil {
				return r, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return r, fmt.Errorf("ctx-size: %w", err)
			}
			r.ctxSize = n
			i++
		case "-ngl", "--n-gpu-layers", "--gpu-layers":
			v, err := need(i, toks[i])
			if err != nil {
				return r, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return r, fmt.Errorf("n-gpu-layers: %w", err)
			}
			r.nGPULayers = n
			i++
		case "-ts", "--tensor-split":
			v, err := need(i, toks[i])
			if err != nil {
				return r, err
			}
			r.tensorSplit = v
			i++
		case "-ot", "--override-tensor":
			v, err := need(i, toks[i])
			if err != nil {
				return r, err
			}
			r.overrideTensor = v
			i++
		default:
			return r, fmt.Errorf(tr("unknown argument in fit-params output: %q"), toks[i])
		}
	}
	if r.ctxSize <= 0 {
		return r, fmt.Errorf(tr("fit-params output has no -c (got: %q)"), strings.TrimSpace(s))
	}
	return r, nil
}

func runFit(cfg config, modelPath string, fixed []string) (fitResult, error) {
	args := append([]string{"--model", modelPath}, fixed...)
	cmd := exec.Command(cfg.fitParams, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fitResult{}, fmt.Errorf("llama-fit-params: %w: %s", err, detail)
		}
		return fitResult{}, fmt.Errorf("llama-fit-params: %w", err)
	}
	return parseFitOutput(stdout.String())
}

func runFitPrint(cfg config, modelPath string, fixed []string) (string, error) {
	args := append([]string{"--model", modelPath, "--fit-print", "on"}, fixed...)
	cmd := exec.Command(cfg.fitParams, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("llama-fit-params --fit-print: %w: %s", err, detail)
		}
		return "", fmt.Errorf("llama-fit-params --fit-print: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// parseTuneArgs: первый не-флаг — имя; пары --ключ=значение / --ключ значение.
// Ключ --ctx раскрывается в ctx-size. Возвращает имя, порядок ключей и map.
func parseTuneArgs(args []string) (string, []string, map[string]string, error) {
	var name string
	var order []string
	m := make(map[string]string)
	put := func(k, v string) {
		if _, ok := m[k]; !ok {
			order = append(order, k)
		}
		m[k] = v
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--"):
			k, v, eq := strings.Cut(strings.TrimPrefix(a, "--"), "=")
			if k == "ctx" {
				k = "ctx-size"
			}
			if !eq {
				if i+1 >= len(args) {
					return "", nil, nil, fmt.Errorf(tr("missing value after --%s"), k)
				}
				i++
				v = args[i]
			}
			if v == "" {
				return "", nil, nil, fmt.Errorf(tr("empty value for --%s"), k)
			}
			put(k, v)
		case name == "":
			name = a
		default:
			return "", nil, nil, fmt.Errorf(tr("unexpected argument: %s"), a)
		}
	}
	if name == "" {
		return "", nil, nil, errors.New(tr("specify model name: llama tune <name> [--key value]"))
	}
	return name, order, m, nil
}

func modelPath(cfg config, name string) string {
	return cfg.modelsDir + string(os.PathSeparator) + name + ".gguf"
}

func cmdTune(cfg config, args []string) error {
	name, order, forced, err := parseTuneArgs(args)
	if err != nil {
		return err
	}
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	path := modelPath(cfg, name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf(tr("model not found: %s"), path)
	}

	// Качество/память: шаблон; вытесняется значениями пользователя.
	template := []kv{
		{"cache-type-k", "q4_0"},
		{"cache-type-v", "q4_0"},
		{"flash-attn", "on"},
		{"parallel", "1"},
		{"threads", strconv.Itoa(max(1, nproc()-2))},
	}
	tmplMap := make(map[string]string, len(template))
	for _, k := range template {
		tmplMap[k.key] = k.value
	}

	// fit-params фиксирует то, что задано явно (пользователь + шаблон памяти KV),
	// и достраивает ctx-size / n-gpu-layers под свободную память устройств.
	var fixed []string
	addFixed := func(k, v string) {
		fixed = append(fixed, "--"+k, v)
	}
	for _, k := range order {
		addFixed(k, forced[k])
	}
	for _, k := range []string{"cache-type-k", "cache-type-v", "flash-attn"} {
		if _, ok := forced[k]; !ok {
			addFixed(k, tmplMap[k])
		}
	}

	if isUnitActive(cfg, activeUnit(cfg)) {
		fmt.Println(tr("server is active: less free VRAM, the estimate may be conservative (prefer tune with the server stopped)"))
	}
	fmt.Println(tr("=== Placement calculation (llama-fit-params) ==="))
	res, err := runFit(cfg, path, fixed)
	if err != nil {
		return err
	}

	ngl := strconv.Itoa(res.nGPULayers)
	switch res.nGPULayers {
	case -1:
		ngl = "auto"
	case -2:
		ngl = "all"
	}
	keys := []kv{
		{"ctx-size", strconv.Itoa(res.ctxSize)},
		{"n-gpu-layers", ngl},
	}
	if res.tensorSplit != "" {
		keys = append(keys, kv{"tensor-split", res.tensorSplit})
	}
	// fit-params материализует --n-cpu-moe/--cpu-moe в явные -ot-паттерны;
	// при заданных пользователем ключах размещения дублировать их в override-tensor не нужно.
	_, forcedMoe := forced["n-cpu-moe"]
	_, forcedCpuMoe := forced["cpu-moe"]
	_, forcedCpuFfn := forced["n-cpu-ffn"]
	if res.overrideTensor != "" && !(forcedMoe || forcedCpuMoe || forcedCpuFfn) {
		keys = append(keys, kv{"override-tensor", res.overrideTensor})
	}
	for _, k := range order {
		keys = append(keys, kv{k, forced[k]})
	}
	for _, k := range template {
		if _, ok := forced[k.key]; !ok {
			keys = append(keys, k)
		}
	}

	if err := upsertSection(cfg.presets, name, keys); err != nil {
		return err
	}
	fmt.Println(tr("preset updated: [%s]", name))
	sec, err := presetShow(cfg.presets, name)
	if err != nil {
		return err
	}
	fmt.Println(sec)
	fmt.Println()
	fmt.Println(tr("Apply: llama restart"))
	return nil
}

func cmdInfo(cfg config, args []string) error {
	if len(args) < 1 || args[0] == "" {
		usage(tr("specify model name: llama info <name>"))
	}
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	name := args[0]
	path := modelPath(cfg, name)
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf(tr("model not found: %s"), path)
	}
	fmt.Printf("═══ %s ═══\n", name)
	fmt.Println(tr("File: %s (%s)", path, humanBytes(st.Size())))

	fmt.Println("── GGUF ──")
	pairs, err := ggufMeta(path)
	if err != nil {
		fmt.Println(tr("(metadata unavailable: %v)", err))
	} else {
		fmt.Println(formatGgufMeta(pairs))
	}

	if out, err := runFitPrint(cfg, path, nil); err == nil && out != "" {
		fmt.Println(tr("=== Memory estimate (MiB: device, model, context, compute) ==="))
		fmt.Println(out)
	} else if err != nil {
		fmt.Println(tr("=== Memory estimate ==="))
		fmt.Println(tr("(unavailable: %v)", err))
	}

	fmt.Println(tr("=== Preset ==="))
	sec, err := presetShow(cfg.presets, name)
	if err != nil {
		return err
	}
	if sec == "" {
		fmt.Println(tr("(none; create: llama tune %s)", name))
	} else {
		fmt.Println(sec)
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
