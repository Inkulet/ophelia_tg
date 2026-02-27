package app

import (
	"fmt"
	"log"
	"time"

	tele "gopkg.in/telebot.v3"
)

// StartScheduler запускает фоновый процесс проверки времени
func StartScheduler(bot *tele.Bot, wm *WomanManager, chatID int64) {
	log.Println("⏰ Планировщик запущен")

	// Тикер срабатывает каждую минуту
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// 1. Проверка ежедневного поста
		checkAndSend(bot, wm, chatID)

		// 2. Проверка еженедельного бэкапа
		checkAndBackup(bot, wm)

		// 3. Личные подписки
		checkAndSendSubscriptions(bot, wm)

		// 4. Тематический пост недели
		checkAndSendTheme(bot, wm, chatID)

		// 5. Здоровье бота
		checkAndSendHealth(bot, wm)

		// 6. Еженедельный отчет
		checkAndSendReport(bot, wm)
	}
}

func checkAndSend(bot *tele.Bot, wm *WomanManager, chatID int64) {
	// 1. Получаем настройки
	settings, err := wm.GetSettings()
	if err != nil {
		log.Println("❌ Ошибка чтения настроек планировщика:", err)
		return
	}

	// Если выключено - выходим
	if !settings.IsActive {
		return
	}

	// 2. Проверяем, отправляли ли уже сегодня
	now := time.Now()
	// Если год и день совпадают с последним запуском — выходим
	if settings.LastRun.Year() == now.Year() && settings.LastRun.YearDay() == now.YearDay() {
		return
	}

	// 3. Сравниваем время (HH:MM)
	targetTime, err := time.Parse("15:04", settings.ScheduleTime)
	if err != nil {
		log.Println("⚠️ Неверный формат времени в БД:", settings.ScheduleTime)
		return
	}

	// Если текущий час и минута совпадают с целевым
	if now.Hour() == targetTime.Hour() && now.Minute() == targetTime.Minute() {
		log.Printf("🔔 Время пришло! (%s). Отправляем случайную героиню...", settings.ScheduleTime)

		// 4. Выбираем случайную героиню
		woman := wm.GetRandomWoman()
		if woman == nil {
			log.Println("⚠️ База пуста, нечего отправлять.")
			return
		}

		// 5. Отправляем в канал
		channel := &tele.Chat{ID: chatID}
		err := sendWithRetry(3, 500*time.Millisecond, func() error {
			return wm.SendWomanCard(bot, channel, woman)
		})
		if err != nil {
			log.Println("❌ Ошибка автоматической отправки:", err)
			return
		}

		// 6. Обновляем дату последнего запуска
		settings.LastRun = now
		if err := wm.UpdateSettings(settings); err != nil {
			log.Printf("⚠️ Не удалось обновить LastRun: %v", err)
		}
		log.Println("✅ Автоматическая рассылка выполнена успешно.")
	}
}

// checkAndBackup проверяет, нужно ли делать бэкап (Раз в неделю, Воскресенье, 03:00)
func checkAndBackup(bot *tele.Bot, wm *WomanManager) {
	now := time.Now()

	// Условие: Воскресенье И время 03:00 (ночи) И 00 минут
	if now.Weekday() == time.Sunday && now.Hour() == 3 && now.Minute() == 0 {
		// Небольшая защита от повторного запуска в ту же минуту (можно через sleep или флаг, но тут просто лог)
		log.Println("💾 Время еженедельного бэкапа...")
		PerformBackup(bot, wm)
		// Ждем минуту, чтобы не отправить дважды в течение 03:00
		time.Sleep(61 * time.Second)
	}
}

// PerformBackup выполняет сжатие и отправку базы данных
func PerformBackup(bot *tele.Bot, wm *WomanManager) {
	// 1. Оптимизация базы данных перед отправкой
	if err := wm.Vacuum(); err != nil {
		log.Printf("⚠️ Ошибка Vacuum перед бэкапом: %v", err)
	}

	// 2. Формирование файла
	file := &tele.Document{
		File:     tele.FromDisk(wm.FilePath),
		Caption:  fmt.Sprintf("💾 <b>Авто-Бэкап базы данных</b>\n📅 %s\n📦 <i>Weekly Backup</i>", time.Now().Format("02.01.2006 15:04")),
		FileName: "women_backup.db",
	}

	// 3. Отправка всем админам
	// Переменная admins должна быть определена в moderation.go или main.go
	adminIDs := getAdmins()
	if len(adminIDs) == 0 {
		log.Println("⚠️ Нет админов для отправки бэкапа.")
		return
	}

	for _, adminID := range adminIDs {
		_, err := bot.Send(&tele.User{ID: adminID}, file, tele.ModeHTML)
		if err != nil {
			log.Printf("⚠️ Не удалось отправить бэкап админу %d: %v", adminID, err)
		} else {
			log.Printf("✅ Бэкап отправлен админу %d", adminID)
		}
	}
}

// Личные подписки
func checkAndSendSubscriptions(bot *tele.Bot, wm *WomanManager) {
	now := time.Now()
	subs := wm.ListActiveSubscriptions()
	if len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		if !sub.IsActive {
			continue
		}
		if sub.LastRun.Year() == now.Year() && sub.LastRun.YearDay() == now.YearDay() {
			continue
		}
		targetTime, err := time.Parse("15:04", sub.Time)
		if err != nil {
			continue
		}
		if now.Hour() == targetTime.Hour() && now.Minute() == targetTime.Minute() {
			w := wm.GetRandomWoman()
			if w == nil {
				continue
			}
			err := sendWithRetry(3, 500*time.Millisecond, func() error {
				_, e := bot.Send(&tele.User{ID: sub.UserID}, "🕯 <b>Ежедневная история</b>", tele.ModeHTML)
				return e
			})
			if err == nil {
				_ = sendWithRetry(3, 500*time.Millisecond, func() error {
					return wm.SendWomanCard(bot, &tele.User{ID: sub.UserID}, w)
				})
			}
			sub.LastRun = now
			_ = wm.UpdateSubscription(&sub)
		}
	}
}

// Тематический пост
func checkAndSendTheme(bot *tele.Bot, wm *WomanManager, chatID int64) {
	s, err := wm.GetSettings()
	if err != nil || s == nil {
		return
	}
	if !s.ThemeActive {
		return
	}
	now := time.Now()
	if s.ThemeLastRun.Year() == now.Year() && s.ThemeLastRun.YearDay() == now.YearDay() {
		return
	}
	if int(now.Weekday()) != s.ThemeWeekday {
		return
	}
	targetTime, err := time.Parse("15:04", s.ThemeTime)
	if err != nil {
		return
	}
	if now.Hour() != targetTime.Hour() || now.Minute() != targetTime.Minute() {
		return
	}
	theme := pickWeeklyTheme()
	if theme == "" {
		return
	}
	channel := &tele.Chat{ID: chatID}
	err = sendWithRetry(3, 500*time.Millisecond, func() error {
		_, e := bot.Send(channel, fmt.Sprintf("🗝 <b>Тема недели:</b> %s\nТри голоса из летописи.", theme), tele.ModeHTML)
		return e
	})
	if err != nil {
		return
	}
	items := wm.GetRandomWomenByField(theme, 3)
	for _, w := range items {
		_ = sendWithRetry(3, 500*time.Millisecond, func() error {
			return wm.SendWomanCard(bot, channel, &w)
		})
		time.Sleep(120 * time.Millisecond)
	}
	s.ThemeLastRun = now
	_ = wm.UpdateSettings(s)
}

// Ежедневный health report
func checkAndSendHealth(bot *tele.Bot, wm *WomanManager) {
	s, err := wm.GetSettings()
	if err != nil || s == nil {
		return
	}
	if !s.HealthActive {
		return
	}
	now := time.Now()
	if s.HealthLastRun.Year() == now.Year() && s.HealthLastRun.YearDay() == now.YearDay() {
		return
	}
	targetTime, err := time.Parse("15:04", s.HealthTime)
	if err != nil {
		return
	}
	if now.Hour() != targetTime.Hour() || now.Minute() != targetTime.Minute() {
		return
	}
	status := buildStatusText()
	audit := buildAuditReport()
	for _, adminID := range getAdmins() {
		_ = sendWithRetry(3, 500*time.Millisecond, func() error {
			_, e := bot.Send(&tele.User{ID: adminID}, status, tele.ModeHTML)
			return e
		})
		_ = sendWithRetry(3, 500*time.Millisecond, func() error {
			_, e := bot.Send(&tele.User{ID: adminID}, audit, tele.ModeHTML)
			return e
		})
	}
	s.HealthLastRun = now
	_ = wm.UpdateSettings(s)
}

func checkAndSendReport(bot *tele.Bot, wm *WomanManager) {
	s, err := wm.GetSettings()
	if err != nil || s == nil {
		return
	}
	if !s.ReportActive {
		return
	}
	now := time.Now()
	if s.ReportLastRun.Year() == now.Year() && s.ReportLastRun.YearDay() == now.YearDay() {
		return
	}
	if int(now.Weekday()) != s.ReportWeekday {
		return
	}
	targetTime, err := time.Parse("15:04", s.ReportTime)
	if err != nil {
		return
	}
	if now.Hour() != targetTime.Hour() || now.Minute() != targetTime.Minute() {
		return
	}
	report := buildWeeklyReport()
	for _, adminID := range getAdmins() {
		_ = sendWithRetry(3, 500*time.Millisecond, func() error {
			_, e := bot.Send(&tele.User{ID: adminID}, report, tele.ModeHTML)
			return e
		})
	}
	s.ReportLastRun = now
	_ = wm.UpdateSettings(s)
}
