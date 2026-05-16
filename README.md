# Tree Shield NewsBot

Пятничный дайджест: RSS за неделю → Gemini → **превью в личку** (в канал копируете сами).

```bash
cp .env.example .env
make build
./treesheild-newsbot -preview   # один раз
./treesheild-newsbot            # планировщик (пятница 18:00 MSK)
```

1. Напишите боту `/start`.
2. После `-preview` или по cron — **второе** сообщение в личке → копируете в канал.

## VPS

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot && sudo ./deploy/install.sh
sudo nano /opt/treesheild-newsbot/.env
sudo systemctl enable --now treesheild-newsbot
journalctl -u treesheild-newsbot -f
```

Обновление: `git pull && sudo ./deploy/install.sh && sudo systemctl restart treesheild-newsbot`

## Переменные

| Переменная | Описание |
|------------|----------|
| `TELEGRAM_BOT_TOKEN` | @BotFather |
| `TELEGRAM_PREVIEW_CHAT_ID` | Ваш chat id |
| `GEMINI_API_KEY` | Google AI Studio |
| `GEMINI_MODEL` | `gemini-2.5-flash` |
| `TZ` | `Europe/Moscow` |
| `CRON_SCHEDULE` | `0 18 * * 5` |
