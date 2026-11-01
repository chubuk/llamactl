# llamactl

CLI-менеджер для локального сервера [llama.cpp](https://github.com/ggml-org/llama.cpp):
systemd-юниты (обычный режим и MCP-proxy), скачивание моделей с HuggingFace
и **автоматический подбор пресетов под ваш GPU/CPU** через `llama-fit-params`.

Команда, которую вы набираете, — `llama` (как `rg` из ripgrep: репозиторий
`llamactl`, бинарь можно установить под именем `llama`).

**Languages: [English](README.md) | Русский**

## Возможности

- **Управление сервером** — `start`, `start-proxy`, `stop`, `restart`, `status`, `log`, `tail`, `health`
- **Работа с моделями** — `list`, `add` (HuggingFace), `remove`, `disable` / `enable`
- **`llama tune`** — генерирует записи `presets.ini` (`ctx-size`, `n-gpu-layers`,
  гибридное размещение MoE через `--n-cpu-moe`, типы KV-кэша) по свободной
  памяти *вашей* машины — расчёт выполняет `llama-fit-params`
- **`llama info`** — метаданные GGUF, оценка памяти и текущий пресет модели
- **Proxy-режим** — `--ui-mcp-proxy` (CORS-прокси, чтобы встроенный Web UI мог
  обращаться к удалённым MCP-серверам) отдельным systemd-юнитом с сохранением
  режима после перезагрузки
- **Безопасно по умолчанию** — сервер слушает `127.0.0.1`
- **Интерфейс на русском и английском** — по локали (`LANG`) или через
  `lang` / `LLAMA_LANG`

## Требования

- Linux с `systemd --user`
- локально собранная llama.cpp, включая `llama-server` и `llama-fit-params`
  (путь по умолчанию: `~/llama.cpp/build/bin`)
- CLI [`hf`](https://huggingface.co/docs/huggingface_hub/guides/cli) для скачивания моделей
- Go ≥ 1.26 (только для сборки)

## Установка

Из исходников:

```sh
git clone https://github.com/chubuk/llamactl && cd llamactl
mkdir -p ~/bin
go build -o ~/bin/llama .
```

Через `go install` (бинарь называется `llamactl`; сделайте симлинк или
переименуйте его в `llama`):

```sh
go install github.com/chubuk/llamactl@latest
mkdir -p ~/bin
ln -s "$(go env GOPATH)/bin/llamactl" ~/bin/llama
```

Пакеты `.deb` и `.rpm` приложены к [релизам](https://github.com/chubuk/llamactl/releases).

## systemd-юниты

Менеджер управляет двумя взаимоисключающими user-юнитами (см. [`deploy/`](deploy/)):

| Юнит | Назначение |
|---|---|
| `llama-server.service` | обычный режим |
| `llama-server-proxy.service` | тот же состав + `--ui-mcp-proxy` (CORS-прокси для Web UI) |

Установите их в `~/.config/systemd/user/`, поправьте пути внутри, затем:

```sh
systemctl --user daemon-reload
llama start          # или: llama start-proxy
llama enable-svc     # автозапуск текущего режима (переживает перезагрузку)
```

Режим — литеральный CLI-флаг выбранного юнита, никаких env-костылей.

## Конфигурация

Необязательный файл: `~/.config/llamactl/config.ini` (или `$LLAMA_CONFIG`).
Формат — `key = value`, комментарии `#`; см. [`config.ini.example`](config.ini.example).
Приоритет: переменные окружения (`LLAMA_*`) > файл конфигурации > значения по
умолчанию. Команда `llama config` показывает каждое значение и его источник.

| Ключ | Env | По умолчанию |
|---|---|---|
| `models-dir` | `LLAMA_MODELS_DIR` | `~/models` |
| `disabled-dir` | `LLAMA_DISABLED_DIR` | `~/models-disabled` |
| `presets` | `LLAMA_PRESETS` | `<models-dir>/presets.ini` |
| `port` | `LLAMA_PORT` | `8033` |
| `fit-params` | `LLAMA_FIT_PARAMS` | `~/llama.cpp/build/bin/llama-fit-params` |
| `service` | `LLAMA_SERVICE` | `llama-server` |
| `editor` | `VISUAL`/`EDITOR` | `nano` |
| `lang` | `LLAMA_LANG` | по локали (`en`/`ru`) |

## Команды

| Команда | Описание |
|---|---|
| `llama start` / `start-proxy` | запуск в обычном / MCP-proxy режиме |
| `llama stop` / `restart` / `status` | жизненный цикл сервиса |
| `llama log` / `tail` / `health` | логи (live / последние 50), health API + текущий режим |
| `llama list` | установленные, отключённые и загруженные модели |
| `llama add <repo> [файл]` | скачать модель с HuggingFace |
| `llama remove <имя>` | удалить файл модели и пресет (с подтверждением) |
| `llama disable` / `enable <имя>` | убрать модель из работы / вернуть |
| `llama tune <имя> [--ключ знач.]` | автоподбор пресета под ваше железо |
| `llama info <имя>` | метаданные GGUF, оценка памяти, пресет |
| `llama preset <имя>` / `presets` | показать пресет / открыть `presets.ini` в редакторе |
| `llama config` | показать действующие настройки и их источники |
| `llama edit` / `reload` | отредактировать активный юнит / `daemon-reload` |
| `llama enable-svc` / `disable-svc` | автозапуск текущего режима вкл/выкл |

Примеры:

```sh
llama add bartowski/Qwen2.5-Coder-14B-Instruct-GGUF Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf
llama tune Qwen2.5-Coder-14B-Instruct-Q4_K_M
llama tune Qwen3.6-35B-A3B-UD-Q4_K_M --n-cpu-moe 28   # гибрид MoE: эксперты в RAM
llama add org/model file.gguf --ctx 32768              # без интерактивного запроса
llama start-proxy
```

История релизов — в [CHANGELOG.md](CHANGELOG.md).

## Как работает `tune`

`tune` запускает `llama-fit-params` для GGUF-файла и свободной памяти устройств,
затем записывает результат в `presets.ini` (существующие пользовательские ключи,
например `chat-template-kwargs`, сохраняются). Значения, которые вы передали
(`--n-cpu-moe`, `--ctx`, …), считаются зафиксированными и попадают в пресет
как есть. Качественные значения по умолчанию: `cache-type-k/v = q4_0`,
`flash-attn = on`, `parallel = 1`, `threads = nproc-2`. Для больших MoE-моделей,
не влезающих в VRAM, передайте `--n-cpu-moe N` — веса экспертов останутся
в RAM, а внимание будет работать на GPU.

## Безопасность

По умолчанию сервер слушает `127.0.0.1`. Флаг `--ui-mcp-proxy` включает
`/cors-proxy` — открытый HTTP-форвардер, предназначенный только для того,
чтобы встроенный Web UI обращался к удалённым MCP-серверам; документация
llama.cpp прямо предупреждает *не включать* его в недоверенной сети (риск
SSRF). Не выставляйте порт в сеть без дополнительной защиты (firewall,
reverse proxy с аутентификацией).

## Разработка

```sh
go fmt ./... && go vet ./... && go test ./...
go build -o llama .
```

Пакеты релиза (`.deb`/`.rpm`, amd64/arm64) собираются GoReleaser:

```sh
goreleaser release --snapshot --clean   # локальная сборка в dist/
```

## Лицензия

[MIT](LICENSE)
