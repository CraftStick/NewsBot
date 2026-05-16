# Tree Shield NewsBot

Автономный бот пятничного дайджеста для Telegram-канала Tree Shield VPN: RSS за неделю → Gemini → HTML-пост (только текст).

## Локально

```bash
cp .env.example .env   # заполните ключи
make build
./treesheild-newsbot -preview    # тест в личку с ботом
./treesheild-newsbot -run-once   # разовая публикация в канал
./treesheild-newsbot             # планировщик: пятница 18:00 (TZ из .env)
```

Бот должен быть **админом канала** с правом публикации сообщений.

## Деплой на VPS (systemd)

На сервере нужны **Go 1.24+** или заранее собранный бинарник (`make build-linux` на Mac/CI, затем `scp`).

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
sudo ./deploy/install.sh
sudo nano /opt/treesheild-newsbot/.env
sudo -u newsbot /opt/treesheild-newsbot/treesheild-newsbot -preview
sudo systemctl start treesheild-newsbot
sudo systemctl status treesheild-newsbot
journalctl -u treesheild-newsbot -f
```

Обновление после `git pull`:

```bash
cd NewsBot
sudo ./deploy/install.sh
sudo systemctl restart treesheild-newsbot
```

## Docker (опционально)

```bash
docker build -t treesheild-newsbot .
docker run -d --name newsbot --restart unless-stopped \
  --env-file .env \
  treesheild-newsbot
```

Превью / разовый запуск:

```bash
docker run --rm --env-file .env treesheild-newsbot -preview
docker run --rm --env-file .env treesheild-newsbot -run-once
```

## Переменные окружения

| Переменная | Назначение |
|------------|------------|
| `TELEGRAM_BOT_TOKEN` | Токен @BotFather |
| `TELEGRAM_CHANNEL_ID` | Канал: `-100…` или `@username` |
| `TELEGRAM_PREVIEW_CHAT_ID` | Ваш chat id для `-preview` |
| `GEMINI_API_KEY` | Ключ Google AI Studio |
| `GEMINI_MODEL` | По умолчанию `gemini-2.5-flash` |
| `TZ` | По умолчанию `Europe/Moscow` |

Файл `.env` не коммитить.
