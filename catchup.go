package main

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// catchUpWindow — насколько «просроченный» дайджест ещё имеет смысл досылать.
// 72 ч закрывает типичный сценарий «ребут в пятницу вечером → сервер встал в
// выходные»: дайджест за неделю остаётся актуальным. Более старый пропускаем.
const catchUpWindow = 72 * time.Hour

// lastMissedSlot — самый поздний слот расписания в интервале (since, now].
// Если такой есть, значит с момента последней отправки расписание срабатывало,
// а дайджест не ушёл (бот лежал) — это и есть пропуск.
func lastMissedSlot(sched cron.Schedule, since, now time.Time) (time.Time, bool) {
	var last time.Time
	found := false
	for t := sched.Next(since); !t.After(now); t = sched.Next(t) {
		last = t
		found = true
		if found && now.Sub(last) < 0 {
			break // защита от неожиданностей парсера
		}
	}
	return last, found
}

// runCatchUpIfMissed при старте демона проверяет, не пропущен ли пятничный слот
// из-за простоя, и досылает дайджест. Дедуп — по файлу .last_digest, поэтому
// частые ребуты не приведут к нескольким рассылкам.
func runCatchUpIfMissed(cfg Config) {
	now := time.Now().In(cfg.Timezone)

	lastSent, ok := readDigestSent(cfg.Timezone)
	if !ok {
		// Первый запуск (файла нет): фиксируем базовую точку без рассылки,
		// чтобы свежий деплой не выстрелил дайджестом «за прошлую пятницу».
		if err := recordDigestSent(now); err != nil {
			log.Printf("навёрстывание: не записал отметку старта: %v", err)
		}
		return
	}

	sched, err := cron.ParseStandard(cfg.CronSchedule)
	if err != nil {
		log.Printf("навёрстывание: не разобрал cron %q: %v", cfg.CronSchedule, err)
		return
	}

	slot, found := lastMissedSlot(sched, lastSent, now)
	if !found {
		return // с последней отправки слотов не было — всё вовремя
	}
	if now.Sub(slot) > catchUpWindow {
		log.Printf("навёрстывание: пропущенный слот %s старше %s — пропускаю",
			slot.Format("02.01.2006 15:04"), catchUpWindow)
		return
	}

	log.Printf("навёрстывание: пропущен дайджест за %s — собираю сейчас",
		slot.Format("02.01.2006 15:04"))
	if err := runDigest(cfg); err != nil {
		log.Printf("навёрстывание не удалось: %v", err)
		notifyDigestFailure(cfg, err)
	}
}
