package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type config struct {
	home        string
	modelsDir   string
	disabledDir string
	presets     string
	port        int
	fitParams   string
	service     string
	editor      string
}

func loadConfig() (config, error) {
	cfg, _, _, err := resolveSettings()
	return cfg, err
}

func (c config) proxyService() string { return c.service + "-proxy" }

func (c config) unitFile(unit string) string {
	return filepath.Join(c.home, ".config", "systemd", "user", unit+".service")
}

func (c config) baseURL() string { return fmt.Sprintf("http://localhost:%d", c.port) }

func nproc() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func openInEditor(editor, path string) error {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"nano"}
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func usage(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", tr("Error:"), msg)
	fmt.Fprintf(os.Stderr, "%s\n", tr("Example: see llama help"))
	os.Exit(1)
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	var cmdErr error
	switch cmd {
	case "start":
		cmdErr = cmdStart(cfg, false)
	case "start-proxy":
		cmdErr = cmdStart(cfg, true)
	case "stop":
		cmdErr = cmdStop(cfg)
	case "restart":
		cmdErr = cmdRestart(cfg)
	case "status":
		cmdErr = cmdStatus(cfg)
	case "log":
		cmdErr = cmdLog(cfg)
	case "tail":
		cmdErr = cmdTail(cfg)
	case "health":
		cmdErr = cmdHealth(cfg)
	case "list":
		cmdErr = cmdList(cfg)
	case "add":
		cmdErr = cmdAdd(cfg, args)
	case "remove":
		cmdErr = cmdRemove(cfg, args)
	case "disable":
		cmdErr = cmdDisable(cfg, args)
	case "enable":
		cmdErr = cmdEnable(cfg, args)
	case "tune":
		cmdErr = cmdTune(cfg, args)
	case "info":
		cmdErr = cmdInfo(cfg, args)
	case "preset":
		cmdErr = cmdPreset(cfg, args)
	case "presets":
		cmdErr = cmdPresets(cfg)
	case "config":
		cmdErr = cmdConfig()
	case "edit":
		cmdErr = cmdEdit(cfg)
	case "reload":
		cmdErr = cmdReload(cfg)
	case "enable-svc":
		cmdErr = cmdEnableSvc(cfg, true)
	case "disable-svc":
		cmdErr = cmdEnableSvc(cfg, false)
	case "help", "-h", "--help", "/h", "":
		showHelp(cfg)
	default:
		fmt.Fprintf(os.Stderr, "%s\n\n", tr("Unknown command: %s", cmd))
		showHelp(cfg)
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintln(os.Stderr, tr("Error:"), cmdErr)
		os.Exit(1)
	}
}
