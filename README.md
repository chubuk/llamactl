# llamactl

CLI manager for a local [llama.cpp](https://github.com/ggml-org/llama.cpp) server:
systemd user units (normal and MCP-proxy modes), HuggingFace model downloads,
and **model presets automatically tuned to your GPU/CPU** via `llama-fit-params`.

The command you type is `llama` (like `rg` from ripgrep — the repo is `llamactl`,
the binary can be installed as `llama`).

## Features

- **Server control** — `start`, `start-proxy`, `stop`, `restart`, `status`, `log`, `tail`, `health`
- **Model management** — `list`, `add` (HuggingFace), `remove`, `disable` / `enable`
- **`llama tune`** — generates `presets.ini` entries (`ctx-size`, `n-gpu-layers`,
  MoE hybrid placement via `--n-cpu-moe`, KV cache types) computed by `llama-fit-params`
  for the free VRAM/RAM of *your* machine
- **`llama info`** — GGUF metadata, memory estimate and the current preset of a model
- **Proxy mode** — `--ui-mcp-proxy` (CORS proxy so the built-in Web UI can reach remote
  MCP servers) as a separate, reboot-persistent systemd unit
- **Safe by default** — the server listens on `127.0.0.1`

## Requirements

- Linux with `systemd --user`
- llama.cpp built locally, including `llama-server` and `llama-fit-params`
  (default path: `~/llama.cpp/build/bin`)
- [`hf`](https://huggingface.co/docs/huggingface_hub/guides/cli) CLI for model downloads
- Go ≥ 1.26 (build only)

## Install

From source:

```sh
git clone https://github.com/chubuk/llamactl && cd llamactl
go build -o ~/bin/llama .
```

Or with `go install` (binary is named `llamactl`; symlink or rename it to `llama`):

```sh
go install github.com/chubuk/llamactl@latest
ln -s "$(go env GOPATH)/bin/llamactl" ~/bin/llama
```

`.deb` and `.rpm` packages are attached to [releases](https://github.com/chubuk/llamactl/releases).

## systemd units

The manager drives two mutually exclusive user units (see [`deploy/`](deploy/)):

| Unit | Purpose |
|---|---|
| `llama-server.service` | normal mode |
| `llama-server-proxy.service` | same, plus `--ui-mcp-proxy` (CORS proxy for the Web UI) |

Install them into `~/.config/systemd/user/`, adjust the paths inside, then:

```sh
systemctl --user daemon-reload
llama start          # or: llama start-proxy
llama enable-svc     # autostart for the current mode (survives reboots)
```

The mode is a literal CLI flag of the selected unit — no environment tricks.

## Configuration

Optional file: `~/.config/llamactl/config.ini` (or `$LLAMA_CONFIG`).
Format is `key = value`, `#` comments — see [`config.ini.example`](config.ini.example).
Precedence: environment (`LLAMA_*`) > config file > defaults. Run `llama config`
to see every resolved setting and its source.

| Key | Env | Default |
|---|---|---|
| `models-dir` | `LLAMA_MODELS_DIR` | `~/models` |
| `disabled-dir` | `LLAMA_DISABLED_DIR` | `~/models-disabled` |
| `presets` | `LLAMA_PRESETS` | `<models-dir>/presets.ini` |
| `port` | `LLAMA_PORT` | `8033` |
| `fit-params` | `LLAMA_FIT_PARAMS` | `~/llama.cpp/build/bin/llama-fit-params` |
| `service` | `LLAMA_SERVICE` | `llama-server` |
| `editor` | `VISUAL`/`EDITOR` | `nano` |

## Commands

| Command | Description |
|---|---|
| `llama start` / `start-proxy` | start in normal / MCP-proxy mode |
| `llama stop` / `restart` / `status` | service lifecycle |
| `llama log` / `tail` / `health` | logs (live / last 50), API health + current mode |
| `llama list` | installed, disabled and loaded models |
| `llama add <repo> [file]` | download a model from HuggingFace |
| `llama remove <name>` | delete model file + preset (with confirmation) |
| `llama disable` / `enable <name>` | move a model out of / back into rotation |
| `llama tune <name> [--key val]` | auto-compute the preset for your hardware |
| `llama info <name>` | GGUF metadata, memory estimate, preset |
| `llama preset <name>` / `presets` | show a preset / edit `presets.ini` |
| `llama config` | show resolved settings and their sources |
| `llama edit` / `reload` | edit the active unit / `daemon-reload` |
| `llama enable-svc` / `disable-svc` | toggle autostart for the current mode |

Examples:

```sh
llama add bartowski/Qwen2.5-Coder-14B-Instruct-GGUF Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf
llama tune Qwen2.5-Coder-14B-Instruct-Q4_K_M
llama tune Qwen3.6-35B-A3B-UD-Q4_K_M --n-cpu-moe 28   # hybrid MoE: experts in RAM
llama start-proxy
```

## How `tune` works

`tune` runs `llama-fit-params` against the GGUF file and the free memory of your
devices, then writes the result into `presets.ini` (existing custom keys such as
`chat-template-kwargs` are preserved). Values you pass (`--n-cpu-moe`, `--ctx`, …)
are treated as fixed and written to the preset as-is. The quality defaults added
are `cache-type-k/v = q4_0`, `flash-attn = on`, `parallel = 1`,
`threads = nproc-2`. For large MoE models that do not fit in VRAM, pass
`--n-cpu-moe N` to keep expert weights in RAM while attention stays on the GPU.

## Security

The server binds to `127.0.0.1` by default. `--ui-mcp-proxy` enables `/cors-proxy`,
an open HTTP forwarder intended only for the built-in Web UI to reach remote MCP
servers — the upstream llama.cpp docs explicitly warn *not* to enable it in
untrusted environments (SSRF risk). Do not expose the port to the network without
additional protection (firewall, reverse proxy with auth).

## License

[MIT](LICENSE)
