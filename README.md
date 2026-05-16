# Tree Shield NewsBot

**English** · [Русский](README.ru.md)

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Autonomous **Friday IT digest** bot for Telegram: fetches a week of RSS, generates 6 news items with Google Gemini, and sends a ready-made HTML post to your **DM**. You publish to the channel yourself — that way Telegram custom emoji stay animated.

Built for the Tree Shield VPN channel; fork and adapt it for your own project.

---

## Features

- ~20 RSS feeds (Habr, VC, Meduza, Google News, Reddit, etc.) with keyword filtering
- Priority for Russia-related news (RKN, VPN, Telegram, State Duma…)
- 6 digest items: 5 focused on Russia + 1 international story
- Clickable headlines linked to sources
- Post template (header, emoji, closing) in code, not in the LLM prompt
- Retries on Gemini overload (503) and fallback one-item-at-a-time generation
- Cron scheduler, one-shot run, and delayed test (`-in 1m`)

---

## How it works

```
RSS (7 days) → filter → Gemini → HTML body
                              ↓
                    template (format.go)
                              ↓
              2 DMs (hint + digest)
                              ↓
                 you copy to channel
```

---

## Quick start

### Requirements

- Go **1.24+**
- Telegram bot token ([@BotFather](https://t.me/BotFather))
- [Google AI Studio](https://aistudio.google.com/apikey) API key

### Setup

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
cp .env.example .env   # fill in variables
make build
```

Send **`/start`** to your bot, then get your `chat id` via [getUpdates](https://core.telegram.org/bots/api#getupdates).

### Commands

| Command | Description |
|---------|-------------|
| `./treesheild-newsbot -preview` | Build digest and send preview to DM now |
| `./treesheild-newsbot -in 1m` | Same, after 1 minute (test) |
| `./treesheild-newsbot` | Background scheduler (`CRON_SCHEDULE`) |
| `./treesheild-newsbot -cron '0 18 * * 5'` | Override cron for this run |

Copy the **second** message in DM into your channel.

---

## Configuration (`.env`)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | yes | — | Bot token |
| `TELEGRAM_PREVIEW_CHAT_ID` | yes | — | Your numeric chat id |
| `GEMINI_API_KEY` | yes | — | Gemini API key |
| `GEMINI_MODEL` | no | `gemini-2.5-flash` | Model name |
| `TZ` | no | `Europe/Moscow` | Timezone for cron |
| `CRON_SCHEDULE` | no | `0 18 * * 5` | Friday 18:00 |

Cron examples (5 fields: minute hour dom month dow):

```env
CRON_SCHEDULE=0 18 * * 5    # Friday 18:00
CRON_SCHEDULE=*/5 * * * *   # every 5 minutes (testing only)
```

On a server, `.env` is loaded next to the binary (`/opt/treesheild-newsbot/.env`).

---

## VPS deploy (Ubuntu/Debian)

```bash
git clone https://github.com/CraftStick/NewsBot.git
cd NewsBot
sudo ./deploy/install.sh
sudo nano /opt/treesheild-newsbot/.env
sudo -u newsbot /opt/treesheild-newsbot/treesheild-newsbot -preview
sudo systemctl enable --now treesheild-newsbot
journalctl -u treesheild-newsbot -f
```

Update:

```bash
cd NewsBot && git pull && sudo ./deploy/install.sh
sudo systemctl restart treesheild-newsbot
```

Cross-compile for a VPS without Go:

```bash
make build-linux
scp treesheild-newsbot user@server:/opt/treesheild-newsbot/
```

---

## Docker (optional)

```bash
cp .env.example .env
docker build -t treesheild-newsbot .
docker run --rm --env-file .env treesheild-newsbot -preview
```

Scheduler: `docker run -d --restart unless-stopped --env-file .env treesheild-newsbot`

---

## Customization

| What to change | File |
|----------------|------|
| RSS feeds and keywords | `rss.go` |
| Gemini system prompt | `config.go` (`systemPrompt`) |
| Post header, emoji, layout | `format.go` |
| Item count, text length limits | `format.go`, `gemini.go` |

Custom `<tg-emoji>` IDs only show as animated in a channel when you **paste manually** from a Premium account; the Bot API falls back to standard emoji in channel posts.

---

## Development

```bash
make check    # tests + go vet
make preview  # build and run -preview
```

```
├── main.go          # CLI, scheduler
├── config.go        # .env, prompts
├── rss.go           # feeds and filters
├── gemini.go        # generation
├── format.go        # post template
├── links.go         # source URLs
├── telegram.go      # DM delivery
└── deploy/          # systemd + install.sh
```

---

## Security

- Never commit `.env` or tokens (listed in `.gitignore`)
- On VPS: `chmod 600 /opt/treesheild-newsbot/.env`
- Rotate keys in @BotFather and Google AI Studio if leaked

---

## License

[MIT](LICENSE) — use and modify freely with attribution.
