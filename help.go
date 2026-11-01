package main

import "fmt"

func showHelp(cfg config) {
	fmt.Printf(`╔══════════════════════════════════════════════════════════════════╗
║  llama — управление llama.cpp сервером и моделями                ║
╚══════════════════════════════════════════════════════════════════╝

СЕРВЕР:
    start               Запустить сервер в обычном режиме
    start-proxy         Запустить сервер с флагом --ui-mcp-proxy
                        (CORS-прокси Web UI для удалённых MCP-серверов)
    stop                Остановить сервер
    restart             Перезапустить (после правки конфига/пресетов)
    status              Статус сервера (включая режим proxy)
    log                 Логи live (Ctrl+C для выхода)
    tail                Последние 50 строк лога
    health              Проверить API и текущий режим

МОДЕЛИ:
    list                     Показать все установленные модели
    add <repo> [файл]        Скачать модель с HuggingFace
                             пример: llama add bartowski/Qwen2.5-Coder-7B-GGUF Qwen2.5-Coder-7B-Q4_K_M.gguf
    remove <имя>             Удалить модель (файл + пресет)
    disable <имя>            Переместить модель в models-disabled
    enable <имя>             Вернуть модель из models-disabled
    tune <имя> [--ключ N]    Автоподбор пресета по памяти GPU/CPU (llama-fit-params)
                             пример: llama tune Qwen3.6-35B-A3B-UD-Q4_K_M --n-cpu-moe 28
    info <имя>               Параметры модели, оценка памяти, текущий пресет
    preset <имя>             Показать пресет для модели
    presets                  Открыть файл пресетов в редакторе

ПРОЧЕЕ:
    config        Показать действующие настройки и их источники
    edit          Редактировать активный сервис systemd
    reload        Перечитать конфиг systemd
    enable-svc    Включить автозапуск сервера (текущий режим)
    disable-svc   Отключить автозапуск сервера
    help, -h, --help, /h   Эта справка

ПРИМЕРЫ:
    llama add bartowski/Qwen2.5-Coder-14B-Instruct-GGUF Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf
    llama tune Qwen2.5-Coder-14B-Instruct-Q4_K_M
    llama remove Qwen2.5-Coder-7B-Instruct-Q4_K_M
    llama list
    llama start-proxy

РЕЖИМ MCP PROXY:
    Флаг --ui-mcp-proxy включает /cors-proxy — CORS-прокси для встроенного
    Web UI (http://localhost:%[1]d), чтобы браузерный MCP-клиент UI мог
    обращаться к удалённым MCP-серверам. Это НЕ MCP-сервер: редакторы
    (Cursor, VS Code) нельзя подключить как MCP напрямую к localhost:%[1]d.
    ВНИМАНИЕ: /cors-proxy — открытый HTTP-форвардер (риск SSRF),
    не включайте его в недоверенной сети.

КЛИЕНТЫ (OpenWebUI, Continue и т.п.):
    OpenAI API:   http://localhost:%[1]d/v1
    Web UI:       http://localhost:%[1]d
`, cfg.port)
}
