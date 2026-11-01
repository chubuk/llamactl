# Changelog

## 1.0.2 — 2026-11-01

- `tune` UX fixes:
  - a plain `llama tune <name>` reuses `n-cpu-moe` / `cpu-moe` / `n-cpu-ffn`
    from the existing preset instead of replacing them with a giant
    materialized `override-tensor` line
  - stale `override-tensor` is removed from the section when simple
    placement flags are in effect
  - merged sections no longer accumulate trailing blank lines

## 1.0.1 — 2026-11-01

- Russian documentation: `README.ru.md` (with language links in `README.md`)
- `README.ru.md` is now included in the `.deb`/`.rpm` packages

## 1.0.0 — 2026-11-01

First release.

### Highlights
- CLI manager for a local llama.cpp server: `start` / `start-proxy` / `stop` /
  `restart` / `status` / `log` / `tail` / `health`
- Model management: `list`, `add` (HuggingFace), `remove`, `disable` / `enable`
- `llama tune` — automatic `presets.ini` tuning to your GPU/CPU memory via
  `llama-fit-params` (including hybrid MoE placement with `--n-cpu-moe`)
- `llama info` — GGUF metadata, memory estimate and current preset
- `llama config` — resolved settings with their sources (default / file / env)
- MCP-proxy mode as a separate, reboot-persistent systemd unit
  (`llama-server-proxy.service` with a literal `--ui-mcp-proxy` flag)
- Secure by default: the server binds to `127.0.0.1`
- UI in English and Russian (locale auto-detection, `lang` / `LLAMA_LANG`)
- XDG config file `~/.config/llamactl/config.ini`
- `.deb` / `.rpm` packages (amd64, arm64) via GoReleaser
