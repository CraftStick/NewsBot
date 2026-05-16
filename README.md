# Tree Shield NewsBot

Автономный бот пятничного дайджеста для Telegram-канала Tree Shield VPN: RSS за неделю → Gemini → HTML-пост (только текст).

## Локально

```bash
cp .env.example .env   # заполните ключи
make build
./treesheild-newsbot -preview       # в личку: подсказка + дайджест для копирования в канал
./treesheild-newsbot -run-once      # сразу в канал
./treesheild-newsbot -run-in 1m     # в канал через минуту (один раз)
./treesheild-newsbot                # по расписанию CRON_SCHEDULE из .env
./treesheild-newsbot -cron '*/1 * * * *'   # тест: каждую минуту (перебивает .env)
```

### Публикация вручную (превью → канал)

Удобно, если нужны **анимированные эмодзи** в посте:

1. `sudo -u newsbot /opt/treesheild-newsbot/treesheild-newsbot -preview`
2. В личке с ботом откройте **второе** сообщение (чистый дайджест).
3. Скопируйте и вставьте в канал **с аккаунта с Premium** (или опубликуйте «от имени канала» в клиенте).

Автопостинг в канал ботом: `-run-once` или планировщик (`CRON_SCHEDULE`) — эмодзи будут обычными (🗣 ✅), это ограничение Bot API.

Бот должен быть **админом канала** с правом «Публикация сообщений», если публикуете через `-run-once` / cron.

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
| `TELEGRAM_CHANNEL_ID` | Канал: `-100…` или `@username` |
| `TELEGRAM_PREVIEW_CHAT_ID` | Ваш chat id для `-preview` |
| `GEMINI_API_KEY` | Ключ Google AI Studio |
| `GEMINI_MODEL` | По умолчанию `gemini-2.5-flash` |
| `TZ` | По умолчанию `Europe/Moscow` |
| `CRON_SCHEDULE` | По умолчанию `0 18 * * 5`; для теста `*/1 * * * *` |

Файл `.env` не коммитить.
