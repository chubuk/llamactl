package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runSystemctl(cfg config, stream bool, args ...string) (string, error) {
	full := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", full...)
	if stream {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return "", cmd.Run()
	}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func isUnitActive(cfg config, unit string) bool {
	_, err := runSystemctl(cfg, false, "is-active", "--quiet", unit)
	return err == nil
}

func activeUnit(cfg config) string {
	if isUnitActive(cfg, cfg.proxyService()) {
		return cfg.proxyService()
	}
	return cfg.service
}

func waitReady(cfg config, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := cfg.baseURL() + "/health"
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					cancel()
					return nil
				}
			}
		}
		cancel()
		if time.Now().After(deadline) {
			return fmt.Errorf(tr("server did not respond at %s within %s"), url, timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

// probeMode читает cors_proxy_enabled из /props; ok=false — сервер не ответил.
func probeMode(cfg config) (enabled bool, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.baseURL()+"/props", nil)
	if err != nil {
		return false, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	var props struct {
		CorsProxyEnabled bool `json:"cors_proxy_enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&props); err != nil {
		return false, false
	}
	return props.CorsProxyEnabled, true
}

func modeLabel(cfg config) string {
	enabled, ok := probeMode(cfg)
	if !ok {
		return tr("unknown (server not responding)")
	}
	if enabled {
		return tr("MCP proxy (cors-proxy enabled)")
	}
	return tr("normal (cors-proxy disabled)")
}

func startService(cfg config, proxy bool) error {
	target := cfg.service
	other := cfg.proxyService()
	if proxy {
		target, other = other, target
	}
	if isUnitActive(cfg, other) {
		if out, err := runSystemctl(cfg, false, "stop", other); err != nil {
			return fmt.Errorf(tr("stopping %s: %w: %s"), other, err, strings.TrimSpace(out))
		}
	}
	if out, err := runSystemctl(cfg, false, "restart", target); err != nil {
		return fmt.Errorf(tr("starting %s: %w: %s"), target, err, strings.TrimSpace(out))
	}
	if err := waitReady(cfg, 30*time.Second); err != nil {
		return err
	}
	if proxy {
		if enabled, ok := probeMode(cfg); !ok || !enabled {
			return fmt.Errorf(tr("server started, but --ui-mcp-proxy was not applied (see %s: llama tail)"), target)
		}
	}
	return nil
}

func cmdStart(cfg config, proxy bool) error {
	if err := startService(cfg, proxy); err != nil {
		return err
	}
	unit := cfg.service
	if proxy {
		unit = cfg.proxyService()
	}
	runSystemctl(cfg, true, "status", unit, "--no-pager", "-n", "5")
	fmt.Println()
	fmt.Printf("API: %s/v1\n", cfg.baseURL())
	fmt.Println(tr("Mode: %s", modeLabel(cfg)))
	return nil
}

func cmdStop(cfg config) error {
	if _, err := runSystemctl(cfg, false, "stop", cfg.service, cfg.proxyService()); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	fmt.Println(tr("Stopped"))
	return nil
}

func cmdRestart(cfg config) error {
	unit := activeUnit(cfg)
	if out, err := runSystemctl(cfg, false, "restart", unit); err != nil {
		return fmt.Errorf(tr("restarting %s: %w: %s"), unit, err, strings.TrimSpace(out))
	}
	if err := waitReady(cfg, 30*time.Second); err != nil {
		return err
	}
	runSystemctl(cfg, true, "status", unit, "--no-pager", "-n", "5")
	fmt.Println(tr("Mode: %s", modeLabel(cfg)))
	return nil
}

func cmdStatus(cfg config) error {
	unit := activeUnit(cfg)
	runSystemctl(cfg, true, "status", unit, "--no-pager")
	fmt.Println(tr("Mode: %s", modeLabel(cfg)))
	return nil
}

func cmdLog(cfg config) error {
	return journalctl(cfg, activeUnit(cfg), "-f")
}

func cmdTail(cfg config) error {
	return journalctl(cfg, activeUnit(cfg), "-n", "50", "--no-pager")
}

func journalctl(cfg config, unit string, args ...string) error {
	full := append([]string{"--user", "-u", unit}, args...)
	cmd := exec.Command("journalctl", full...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdHealth(cfg config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.baseURL()+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(tr("Health: not responding"))
		return fmt.Errorf(tr("server unavailable: %w"), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Health: %s\n", strings.TrimSpace(string(body)))
	fmt.Println(tr("Mode: %s", modeLabel(cfg)))
	return nil
}

func cmdReload(cfg config) error {
	if out, err := runSystemctl(cfg, false, "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w: %s", err, strings.TrimSpace(out))
	}
	fmt.Println(tr("Systemd config reloaded"))
	return nil
}

func cmdEnableSvc(cfg config, enable bool) error {
	unit := activeUnit(cfg)
	verb := "disable"
	if enable {
		verb = "enable"
	}
	if out, err := runSystemctl(cfg, false, verb, unit); err != nil {
		return fmt.Errorf("%s %s: %w: %s", verb, unit, err, strings.TrimSpace(out))
	}
	if enable {
		fmt.Println(tr("Autostart enabled for %s", unit))
	} else {
		fmt.Println(tr("Autostart disabled for %s", unit))
	}
	return nil
}

func cmdEdit(cfg config) error {
	unit := activeUnit(cfg)
	return openInEditor(cfg.editor, cfg.unitFile(unit))
}
