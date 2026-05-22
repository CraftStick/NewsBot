# Tree Shield NewsBot

[English](README.md) · **Русский**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Автономный бот **пятничного IT-дайджеста** для Telegram: собирает RSS за неделю, генерирует 6 новостей через Google Gemini и присылает готовый HTML-пост **в личку**. В канал публикуете сами — так сохраняются кастомные эмодзи Telegram.

Изначально сделан для канала Tree Shield VPN; репозиторий можно форкнуть и настроить под свой канал.

---

## Возможности

- ~20 RSS-лент (Habr, VC, Meduza, Google News, Reddit и др.) с фильтром по ключевым словам
- Приоритет новостей про Россию (РКН, VPN, Telegram, Госдума…)
- 6 пунктов дайджеста: 5 про РФ + 1 зарубежная тема
- Кликабельные заголовки со ссылками на источники
- Шаблон поста (шапка, эмодзи, прощание) — в коде, не в промпте
- Повторы при перегрузке Gemini (503) и запасной режим «по одной новости»
- Планировщик cron + разовый запуск и тест с задержкой

---

## Как это работает

```
RSS (7 дней) → фильтр → Gemini → HTML-тело
                              ↓
                    шаблон format.go
                              ↓
              2 сообщения в личку (подсказка + дайджест)
                              ↓
                    вы копируете в канал
```

---

## Быстрый старт

### Требования

- Go **1.24+**
- Токен Telegram-бота ([@BotFather](https://t.me/BotFather))
- API-ключ [Google AI Studio](https://aistudio.google.com/apikey)

### Установка

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
cp .env.example .env   # заполните переменные
make build
```

Напишите боту **`/start`**, узнайте `chat id` через [getUpdates](https://core.telegram.org/bots/api#getupdates).

### Команды

| Команда | Описание |
|---------|----------|
| `./treesheild-newsbot -preview` | Сразу собрать и отправить превью в личку |
| `./treesheild-newsbot -in 1m` | То же через 1 минуту (тест) |
| `./treesheild-newsbot` | Фоновый планировщик по `CRON_SCHEDULE` |
| `./treesheild-newsbot -cron '0 18 * * 5'` | Переопределить cron на один запуск |

После отправки откройте **второе** сообщение в личке — его копируете в канал.

**Не понравилось?** Кнопка **«🔄 Другой дайджест»** под первым сообщением или команда `/digest` — бот соберёт заново (нужен запущенный процесс: `-preview` или `systemctl`).

---

## Конфигурация (`.env`)

| Переменная | Обязательно | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `TELEGRAM_BOT_TOKEN` | да | — | Токен бота |
| `TELEGRAM_PREVIEW_CHAT_ID` | да | — | Ваш числовой chat id |
| `GEMINI_API_KEY` | да | — | Ключ Gemini API |
| `GEMINI_MODEL` | нет | `gemini-2.5-flash` | Модель |
| `TZ` | нет | `Europe/Moscow` | Часовой пояс cron |
| `CRON_SCHEDULE` | нет | `0 18 * * 5` | Пятница 18:00 |

Примеры cron (5 полей):

```env
CRON_SCHEDULE=0 18 * * 5    # пятница 18:00
CRON_SCHEDULE=*/5 * * * *   # каждые 5 минут (только для теста)
```

Файл `.env` на сервере ищется рядом с бинарником (`/opt/treesheild-newsbot/.env`).

---

## Деплой на VPS (Ubuntu/Debian)

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
sudo ./deploy/install.sh
sudo nano /opt/treesheild-newsbot/.env
sudo -u newsbot /opt/treesheild-newsbot/treesheild-newsbot -preview
sudo systemctl enable --now treesheild-newsbot
journalctl -u treesheild-newsbot -f
```

Обновление:

```bash
cd NewsBot && git pull && sudo ./deploy/install.sh
sudo systemctl restart treesheild-newsbot
```

Сборка бинарника на Mac/Linux для VPS без Go:

```bash
make build-linux
scp treesheild-newsbot user@server:/opt/treesheild-newsbot/
```

---

## Docker (опционально)

```bash
cp .env.example .env
docker build -t treesheild-newsbot .
docker run --rm --env-file .env treesheild-newsbot -preview
```

Для планировщика: `docker run -d --restart unless-stopped --env-file .env treesheild-newsbot`

---

## Кастомизация

| Что менять | Файл |
|------------|------|
| RSS-ленты и ключевые слова | `rss.go` |
| Промпт для Gemini | `config.go` (`systemPrompt`) |
| Шапка, эмодзи, оформление поста | `format.go` |
| Число новостей, длина текста | `format.go`, `gemini.go` |

Кастомные эмодзи в `<tg-emoji>` работают в канале только при **ручной** публикации с Premium-аккаунта; Bot API в каналах показывает обычные fallback-emoji.

---

## Разработка

```bash
make check    # тесты + go vet
make preview  # сборка и -preview
```

Структура проекта:

```
├── main.go          # CLI, планировщик
├── config.go        # .env, промпт
├── rss.go           # ленты и фильтры
├── gemini.go        # генерация
├── format.go        # шаблон поста
├── links.go         # ссылки на источники
├── telegram.go      # отправка в личку
└── deploy/          # systemd + install.sh
```

---

## Безопасность

- Не коммитьте `.env` и токены (файл в `.gitignore`)
- На VPS: `chmod 600 /opt/treesheild-newsbot/.env`
- При утечке токена — перевыпустите в @BotFather и Google AI Studio

---

## Лицензия

[MIT](LICENSE) — используйте и изменяйте свободно, с указанием авторства.
