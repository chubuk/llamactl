#!/bin/bash
# llama — управление локальным llama.cpp сервером и моделями

SERVICE="llama-server"
SERVICE_FULL="llama-server.service"
PORT=8033
MODELS_DIR="$HOME/models"
PRESETS="$MODELS_DIR/presets.ini"
DISABLED_DIR="$HOME/models-disabled"

# ==================== ХЕЛП ====================
show_help() {
    cat << 'HELP'
╔══════════════════════════════════════════════════════════════════╗
║  llama — управление llama.cpp сервером и моделями                ║
╚══════════════════════════════════════════════════════════════════╝

СЕРВЕР:
    start               Запустить сервер в обычном режиме
    start-proxy         Запустить сервер в режиме MCP прокси
    stop                Остановить сервер
    restart             Перезапустить (после правки конфига/пресетов)
    status              Статус сервера
    log                 Логи live (Ctrl+C для выхода)
    tail                Последние 50 строк лога
    health              Проверить API

МОДЕЛИ:
    list                     Показать все установленные модели
    add <repo> [файл]        Скачать модель с HuggingFace
                             пример: llama add bartowski/Qwen2.5-Coder-7B-GGUF Qwen2.5-Coder-7B-Q4_K_M.gguf
    remove <имя>             Удалить модель (файл + пресет)
    disable <имя>            Переместить модель в models-disabled
    enable <имя>             Вернуть модель из models-disabled
    preset <имя>             Показать пресет для модели
    presets                  Открыть файл пресетов в редакторе

ПРОЧЕЕ:
    edit          Редактировать сервис systemd
    reload        Перечитать конфиг systemd
    enable        Включить автозапуск сервера
    disable-svc   Отключить автозапуск сервера
    help, -h, --help, /h   Эта справка

ПРИМЕРЫ:
    llama add bartowski/Qwen2.5-Coder-14B-Instruct-GGUF Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf
    llama remove Qwen2.5-Coder-7B-Instruct-Q4_K_M
    llama list
    llama start-proxy

НАСТРОЙКА MCP В РЕДАКТОРАХ:
    
    Cursor / Windsurf:
    1. Откройте настройки редактора
    2. Найдите раздел "MCP Servers" или "Model Context Protocol"
    3. Добавьте новый сервер:
       - Name: local-llama
       - URL: http://localhost:8033
       - Тип: llama.cpp proxy
    4. Сохраните и перезапустите редактор
    
    OpenWebUI:
    1. Перейдите в Settings > Providers > OpenAI API
    2. Укажите:
       - API Base URL: http://localhost:8033
       - API Key: any-key-here
       - Model: Qwen3.6-35B-A3B-UD-Q4_K_M
    3. Сохраните настройки
    
    VS Code с расширением Continue:
    1. Откройте settings.json
    2. Добавьте:
       "continue": {
         "models": [{
           "title": "Local Llama",
           "provider": "openai",
           "model": "Qwen3.6-35B-A3B-UD-Q4_K_M",
           "apiBase": "http://localhost:8033/v1"
         }]
       }
    
    После настройки используйте команду MCP в редакторе, 
    чтобы начать работу с локальной моделью.
HELP
}

# ==================== УТИЛИТЫ ====================
ensure_dirs() {
    mkdir -p "$MODELS_DIR" "$DISABLED_DIR"
    [ -f "$PRESETS" ] || touch "$PRESETS"
}

model_name_from_file() {
    # Убираем путь и расширение .gguf
    basename "$1" .gguf
}

preset_exists() {
    grep -q "^\[$1\]" "$PRESETS"
}

add_preset_if_missing() {
    local name="$1"
    local ctx="${2:-16384}"
    if ! preset_exists "$name"; then
        cat >> "$PRESETS" << EOF

[$name]
ctx-size = $ctx
n-gpu-layers = 99
cache-type-k = q4_0
cache-type-v = q4_0
flash-attn = on
parallel = 1
threads = 10
EOF
        echo "✓ Пресет добавлен: [$name] ctx=$ctx"
    else
        echo "• Пресет уже существует: [$name]"
    fi
}

remove_preset() {
    local name="$1"
    if ! preset_exists "$name"; then
        echo "• Пресет не найден: [$name]"
        return
    fi
    # Удаляем секцию через awk
    awk -v section="[$name]" '
        $0 == section { skip=1; next }
        /^\[/ { skip=0 }
        !skip { print }
    ' "$PRESETS" > "$PRESETS.tmp" && mv "$PRESETS.tmp" "$PRESETS"
    echo "✓ Пресет удалён: [$name]"
}

# ==================== КОМАНДЫ МОДЕЛЕЙ ====================
cmd_list() {
    ensure_dirs
    echo "═══ Активные модели ($MODELS_DIR) ═══"
    if ls "$MODELS_DIR"/*.gguf >/dev/null 2>&1; then
        for f in "$MODELS_DIR"/*.gguf; do
            local name=$(model_name_from_file "$f")
            local size=$(du -h "$f" | cut -f1)
            local preset="✗"
            preset_exists "$name" && preset="✓"
            printf "  %-60s %8s  preset:%s\n" "$name" "$size" "$preset"
        done
    else
        echo "  (пусто)"
    fi

    echo ""
    echo "═══ Отключённые модели ($DISABLED_DIR) ═══"
    if ls "$DISABLED_DIR"/*.gguf >/dev/null 2>&1; then
        for f in "$DISABLED_DIR"/*.gguf; do
            printf "  %s\n" "$(basename "$f")"
        done
    else
        echo "  (пусто)"
    fi

    echo ""
    echo "═══ Загружено в API ═══"
    curl -s "http://localhost:$PORT/v1/models" 2>/dev/null | \
        python3 -c "import json,sys; d=json.load(sys.stdin); [print('  '+m['id']) for m in d.get('data',[])]" 2>/dev/null || \
        echo "  (сервер не запущен)"
}

cmd_add() {
    ensure_dirs
    local repo="$1"
    local file="$2"

    if [ -z "$repo" ]; then
        echo "Ошибка: укажите репозиторий HuggingFace"
        echo "Пример: llama add bartowski/Qwen2.5-Coder-7B-Instruct-GGUF Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf"
        exit 1
    fi

    if ! command -v hf >/dev/null 2>&1; then
        echo "Ошибка: команда 'hf' не найдена. Установите: pip3 install --user huggingface_hub[cli]"
        exit 1
    fi

    echo "═══ Скачивание модели ═══"
    echo "Репозиторий: $repo"
    [ -n "$file" ] && echo "Файл: $file"

    cd "$MODELS_DIR"
    if [ -n "$file" ]; then
        hf download "$repo" "$file" --local-dir "$MODELS_DIR"
    else
        echo "(файл не указан — будет загружен по умолчанию)"
        hf download "$repo" --local-dir "$MODELS_DIR"
    fi

    if [ $? -ne 0 ]; then
        echo "Ошибка скачивания"
        exit 1
    fi

    echo ""
    echo "═══ Новые .gguf в $MODELS_DIR ═══"
    for f in "$MODELS_DIR"/*.gguf; do
        local name=$(model_name_from_file "$f")
        echo "  $name"
    done

    # Определяем имя последней скачанной модели
    local newest=$(ls -t "$MODELS_DIR"/*.gguf 2>/dev/null | head -n1)
    if [ -n "$newest" ]; then
        local newname=$(model_name_from_file "$newest")
        echo ""
        echo "═══ Добавление пресета ═══"
        echo "Введите ctx-size для [$newname] (Enter = 16384):"
        read -r ctx
        ctx="${ctx:-16384}"
        add_preset_if_missing "$newname" "$ctx"
    fi

    echo ""
    echo "Перезапуск сервера..."
    systemctl --user restart "$SERVICE"
    sleep 2
    echo "✓ Готово. Проверьте: llama list"
}

cmd_remove() {
    ensure_dirs
    local name="$1"

    if [ -z "$name" ]; then
        echo "Ошибка: укажите имя модели (без .gguf)"
        echo "Пример: llama remove Qwen2.5-Coder-7B-Instruct-Q4_K_M"
        exit 1
    fi

    local file="$MODELS_DIR/$name.gguf"
    if [ ! -f "$file" ]; then
        echo "Модель не найдена: $file"
        echo "Активные модели: llama list"
        exit 1
    fi

    local size=$(du -h "$file" | cut -f1)
    echo "Удаляемая модель: $name ($size)"
    echo -n "Подтвердить удаление? [y/N]: "
    read -r confirm

    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "Отменено"
        exit 0
    fi

    # Backup пресета
    if preset_exists "$name"; then
        cp "$PRESETS" "$PRESETS.bak"
        remove_preset "$name"
    fi

    rm -f "$file"
    echo "✓ Файл удалён: $file"

    echo "Перезапуск сервера..."
    systemctl --user restart "$SERVICE"
    sleep 2
    echo "✓ Готово"
}

cmd_disable() {
    ensure_dirs
    local name="$1"
    local file="$MODELS_DIR/$name.gguf"

    if [ ! -f "$file" ]; then
        echo "Модель не найдена: $name"
        exit 1
    fi

    mv "$file" "$DISABLED_DIR/"
    echo "✓ Модель перемещена в $DISABLED_DIR"

    # Убираем пресет (сохраняя бэкап)
    if preset_exists "$name"; then
        cp "$PRESETS" "$PRESETS.bak"
        remove_preset "$name"
    fi

    systemctl --user restart "$SERVICE"
    sleep 2
    echo "✓ Готово. Модель отключена"
}

cmd_enable() {
    ensure_dirs
    local name="$1"
    local file="$DISABLED_DIR/$name.gguf"

    if [ ! -f "$file" ]; then
        echo "Модель не найдена в $DISABLED_DIR: $name"
        exit 1
    fi

    mv "$file" "$MODELS_DIR/"
    echo "✓ Модель возвращена в $MODELS_DIR"

    echo "Введите ctx-size для [$name] (Enter = 16384):"
    read -r ctx
    ctx="${ctx:-16384}"
    add_preset_if_missing "$name" "$ctx"

    systemctl --user restart "$SERVICE"
    sleep 2
    echo "✓ Готово"
}

cmd_preset() {
    local name="$1"
    if [ -z "$name" ]; then
        echo "Укажите имя модели: llama preset <имя>"
        exit 1
    fi
    if preset_exists "$name"; then
        awk -v s="[$name]" '$0==s{f=1} /^\[/{if($0!=s)f=0} f' "$PRESETS"
    else
        echo "Пресет не найден для: $name"
    fi
}

# ==================== MAIN ====================
case "$1" in
    start)
        # Устанавливаем переменную окружения в 0 (без прокси)
        systemctl --user set-environment MCP_PROXY=0
        systemctl --user restart "$SERVICE"
        sleep 2
        systemctl --user status "$SERVICE" --no-pager | head -n 5
        echo ""
        echo "API: http://localhost:$PORT/v1"
        ;;
    start-proxy)
        # Устанавливаем переменную окружения в 1 (с прокси)
        systemctl --user set-environment MCP_PROXY=1
        systemctl --user restart "$SERVICE"
        
        sleep 2
        systemctl --user status "$SERVICE" --no-pager | head -n 5
        echo ""
        echo "API: http://localhost:$PORT/v1 (MCP proxy режим)"
        echo "Сервер запущен с флагом --ui-mcp-proxy"
        ;;
    stop)
        systemctl --user stop "$SERVICE"
        echo "Остановлен"
        ;;
    restart)
        systemctl --user restart "$SERVICE"
        sleep 2
        systemctl --user status "$SERVICE" --no-pager | head -n 5
        ;;
    status)
        systemctl --user status "$SERVICE" --no-pager
        ;;
    log)
        journalctl --user -u "$SERVICE" -f
        ;;
    tail)
        journalctl --user -u "$SERVICE" -n 50 --no-pager
        ;;
    health)
        echo -n "Health: "
        curl -s "http://localhost:$PORT/health" || echo "не отвечает"
        echo ""
        ;;
    list)
        cmd_list
        ;;
    add)
        cmd_add "$2" "$3"
        ;;
    remove)
        cmd_remove "$2"
        ;;
    disable)
        cmd_disable "$2"
        ;;
    enable)
        cmd_enable "$2"
        ;;
    preset)
        cmd_preset "$2"
        ;;
    presets)
        "${EDITOR:-nano}" "$PRESETS"
        ;;
    edit)
        "${EDITOR:-nano}" ~/.config/systemd/user/llama-server.service
        ;;
    reload)
        systemctl --user daemon-reload
        echo "Конфиг systemd перечитан"
        ;;
    enable-svc)
        systemctl --user enable "$SERVICE_FULL"
        echo "Автозапуск сервера включён"
        ;;
    disable-svc)
        systemctl --user disable "$SERVICE_FULL"
        echo "Автозапуск сервера отключён"
        ;;
    help|-h|--help|/h|"")
        show_help
        ;;
    *)
        echo "Неизвестная команда: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
