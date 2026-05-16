# Tree Shield NewsBot

Автономный бот пятничного дайджеста для Telegram-канала Tree Shield VPN: RSS за неделю → Gemini → HTML-пост (только текст).

## Локально

```bash
cp .env.example .env   # заполните ключи
make build
./treesheild-newsbot -preview       # сейчас: превью в личку
./treesheild-newsbot                # по CRON — тоже только в личку (DIGEST_MODE=preview)
./treesheild-newsbot -run-once      # принудительно в канал (редко)
```

### Обычный workflow: только превью в личку

По умолчанию `DIGEST_MODE=preview` — бот **не постит в канал**, только присылает дайджест вам в личку. В канал копируете сами.

1. Пятница: systemd шлёт превью (или вручную `-preview`).
2. В личке — **второе** сообщение → копируете в ТГК.

В `.env` на сервере:

```env
DIGEST_MODE=preview
TELEGRAM_PREVIEW_CHAT_ID=ваш_id
# TELEGRAM_CHANNEL_ID можно не указывать
```

Автопост в канал ботом (если когда-нибудь понадобится): `DIGEST_MODE=channel` и `TELEGRAM_CHANNEL_ID`.

### Подключить тестовый канал

1. Создайте канал в Telegram.
2. **Управление каналом → Администраторы → Добавить** вашего бота (с правом публиковать посты).
3. В `.env` укажите канал:
   - публичный: `TELEGRAM_CHANNEL_ID=@your_channel`
   - приватный: id вида `-1001234567890` (перешлите любой пост из канала боту [@RawDataBot](https://t.me/RawDataBot) или смотрите `getUpdates`).
4. Проверка: `./treesheild-newsbot -run-once` или `-run-in 1m`.

### Тест по расписанию (каждую минуту)

В `.env` на сервере:

```env
CRON_SCHEDULE=*/1 * * * *
```

Перезапуск: `sudo systemctl restart treesheild-newsbot`.  
**После теста верните** `CRON_SCHEDULE=0 18 * * 5` (пятница 18:00).

## Деплой на VPS (systemd)

На сервере нужны **Go 1.24+** или заранее собранный бинарник (`make build-linux` на Mac/CI, затем `scp`).

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
sudo ./deploy/install.sh
sudo nano /opt/treesheild-newsbot/.env
cd /opt/treesheild-newsbot && sudo -u newsbot ./treesheild-newsbot -preview
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
| `DIGEST_MODE` | `preview` (по умолчанию) или `channel` |
| `TELEGRAM_CHANNEL_ID` | Нужен только при `DIGEST_MODE=channel` |
| `TELEGRAM_PREVIEW_CHAT_ID` | Ваш chat id для `-preview` |
| `GEMINI_API_KEY` | Ключ Google AI Studio |
| `GEMINI_MODEL` | По умолчанию `gemini-2.5-flash` |
| `TZ` | По умолчанию `Europe/Moscow` |
| `CRON_SCHEDULE` | По умолчанию `0 18 * * 5`; для теста `*/1 * * * *` |

Файл `.env` не коммитить.
