"""Юзербот-публикатор: JSON {"html", "emoji_id"} со stdin → пост в канал с анимированным эмодзи.
Env: TELEGRAM_API_ID, TELEGRAM_API_HASH, PUB_SESSION (путь к .session), PUB_CHANNEL.
`python publish.py --login` — разовый вход (номер, код, 2FA) в терминале."""
import asyncio, json, os, sys
from telethon import TelegramClient
from telethon.extensions import html
from telethon.tl.types import MessageEntityCustomEmoji


async def main():
    c = TelegramClient(os.environ["PUB_SESSION"], int(os.environ["TELEGRAM_API_ID"]), os.environ["TELEGRAM_API_HASH"])
    if "--login" in sys.argv:
        await c.start()
        me = await c.get_me()
        print(f"OK: {me.first_name} (@{me.username})")
        os.chmod(os.environ["PUB_SESSION"] + ".session", 0o600)
        return
    await c.connect()
    if not await c.is_user_authorized():
        sys.exit("сессия не авторизована: нужен --login")
    post = json.load(sys.stdin)
    text, ents = html.parse(post["html"])
    # пост начинается с обычного эмодзи — подменяем его на анимированный (offset/length в UTF-16)
    first = text.split(" ", 1)[0]
    ents.append(MessageEntityCustomEmoji(0, len(first.encode("utf-16-le")) // 2, int(post["emoji_id"])))
    await c.send_message(os.environ["PUB_CHANNEL"], text, formatting_entities=ents, link_preview=False)
    await c.disconnect()


asyncio.run(main())
