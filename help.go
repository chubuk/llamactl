package main

import "fmt"

const helpBody = `╔══════════════════════════════════════════════════════════════════╗
║  llama — %[1]s                                        ║
╚══════════════════════════════════════════════════════════════════╝

%[2]s
    start               %[3]s
    start-proxy         %[4]s
                        %[5]s
    stop                %[6]s
    restart             %[7]s
    status              %[8]s
    log                 %[9]s
    tail                %[10]s
    health              %[11]s

%[12]s
    list                     %[13]s
    add <repo> [file]        %[14]s
                             %[15]s
    remove <name>            %[16]s
    disable <name>           %[17]s
    enable <name>            %[18]s
    tune <name> [--key N]    %[19]s
                             %[20]s
    info <name>              %[21]s
    preset <name>            %[22]s
    presets                  %[23]s

%[24]s
    config        %[25]s
    edit          %[26]s
    reload        %[27]s
    enable-svc    %[28]s
    disable-svc   %[29]s
    help, -h, --help, /h   %[30]s

%[31]s
    llama add bartowski/Qwen2.5-Coder-14B-Instruct-GGUF Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf
    llama tune Qwen2.5-Coder-14B-Instruct-Q4_K_M
    llama remove Qwen2.5-Coder-7B-Instruct-Q4_K_M
    llama list
    llama start-proxy

%[32]s
    %[33]s
    %[34]s
    %[35]s
    %[36]s
    %[37]s
    %[38]s

%[39]s
    OpenAI API:   http://localhost:%[40]d/v1
    Web UI:       http://localhost:%[40]d
`

var helpEn = []any{
	"manage llama.cpp server and models",
	"SERVER:",
	"Start the server in normal mode",
	"Start the server with --ui-mcp-proxy",
	"(CORS proxy of the Web UI for remote MCP servers)",
	"Stop the server",
	"Restart (after editing config/presets)",
	"Server status (including proxy mode)",
	"Live logs (Ctrl+C to exit)",
	"Last 50 log lines",
	"Check API and current mode",
	"MODELS:",
	"Show all installed models",
	"Download a model from HuggingFace",
	"example: llama add bartowski/Qwen2.5-Coder-7B-GGUF Qwen2.5-Coder-7B-Q4_K_M.gguf",
	"Delete a model (file + preset)",
	"Move a model to models-disabled",
	"Return a model from models-disabled",
	"Auto-tune the preset for GPU/CPU memory (llama-fit-params)",
	"example: llama tune Qwen3.6-35B-A3B-UD-Q4_K_M --n-cpu-moe 28",
	"Model parameters, memory estimate, current preset",
	"Show the preset for a model",
	"Open the presets file in the editor",
	"OTHER:",
	"Show resolved settings and their sources",
	"Edit the active systemd service",
	"Reload systemd config",
	"Enable autostart (current mode)",
	"Disable autostart",
	"This help",
	"EXAMPLES:",
	"MCP PROXY MODE:",
	"Flag --ui-mcp-proxy enables /cors-proxy — a CORS proxy for the built-in",
	"Web UI (http://localhost:PORT) so its in-browser MCP client can reach",
	"remote MCP servers. This is NOT an MCP server: editors (Cursor, VS Code)",
	"cannot connect to localhost as MCP directly.",
	"WARNING: /cors-proxy is an open HTTP forwarder (SSRF risk), do not enable",
	"it in untrusted environments.",
	"CLIENTS (OpenWebUI, Continue, etc.):",
}

var helpRu = []any{
	"управление llama.cpp сервером и моделями",
	"СЕРВЕР:",
	"Запустить сервер в обычном режиме",
	"Запустить сервер с флагом --ui-mcp-proxy",
	"(CORS-прокси Web UI для удалённых MCP-серверов)",
	"Остановить сервер",
	"Перезапустить (после правки конфига/пресетов)",
	"Статус сервера (включая режим proxy)",
	"Логи live (Ctrl+C для выхода)",
	"Последние 50 строк лога",
	"Проверить API и текущий режим",
	"МОДЕЛИ:",
	"Показать все установленные модели",
	"Скачать модель с HuggingFace",
	"пример: llama add bartowski/Qwen2.5-Coder-7B-GGUF Qwen2.5-Coder-7B-Q4_K_M.gguf",
	"Удалить модель (файл + пресет)",
	"Переместить модель в models-disabled",
	"Вернуть модель из models-disabled",
	"Автоподбор пресета по памяти GPU/CPU (llama-fit-params)",
	"пример: llama tune Qwen3.6-35B-A3B-UD-Q4_K_M --n-cpu-moe 28",
	"Параметры модели, оценка памяти, текущий пресет",
	"Показать пресет для модели",
	"Открыть файл пресетов в редакторе",
	"ПРОЧЕЕ:",
	"Показать действующие настройки и их источники",
	"Редактировать активный сервис systemd",
	"Перечитать конфиг systemd",
	"Включить автозапуск (текущий режим)",
	"Отключить автозапуск",
	"Эта справка",
	"ПРИМЕРЫ:",
	"РЕЖИМ MCP PROXY:",
	"Флаг --ui-mcp-proxy включает /cors-proxy — CORS-прокси для встроенного",
	"Web UI (http://localhost:PORT), чтобы браузерный MCP-клиент UI мог",
	"обращаться к удалённым MCP-серверам. Это НЕ MCP-сервер: редакторы",
	"(Cursor, VS Code) нельзя подключить как MCP напрямую к localhost.",
	"ВНИМАНИЕ: /cors-proxy — открытый HTTP-форвардер (риск SSRF),",
	"не включайте его в недоверенной сети.",
	"КЛИЕНТЫ (OpenWebUI, Continue и т.п.):",
}

func showHelp(cfg config) {
	args := helpEn
	if uiLang == "ru" {
		args = helpRu
	}
	full := append(args, cfg.port)
	fmt.Printf(helpBody, full...)
}
