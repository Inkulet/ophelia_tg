package app

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ==========================================
// КОНФИГУРАЦИЯ
// ==========================================

type Config struct {
	Token        string `json:"token"`
	GoogleAPI    string `json:"google_api"` // Ключ для GigaChat
	TargetChatID int64  `json:"target_chat_id"`
	BotAPIUrl    string `json:"bot_api_url"`
}

// ==========================================
// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ (Общие для всех файлов)
// ==========================================

var (
	config       Config
	gameManager  *GameManager
	statsManager *StatsManager
	womanManager *WomanManager
)

// ==========================================
// MAIN
// ==========================================

func Run() {
	initAppLayout()
	InitLogger()
	defer CloseLogger()
	markStart()
	rand.Seed(time.Now().UnixNano())

	// 1. Загрузка конфигурации
	if err := loadJSON(configFilePath, &config); err != nil {
		log.Fatalf("❌ Критическая ошибка: не найден или поврежден %s: %v", configFilePath, err)
	}
	applyEnvOverrides(&config)

	// 2. Инициализация Игры (GigaChat)
	var err error
	gameManager, err = InitGame(config.GoogleAPI)
	if err != nil {
		log.Printf("⚠️ Ошибка подключения GigaChat: %v. Игровые функции могут быть ограничены.", err)
	} else {
		log.Println("✅ GigaChat успешно подключен.")
	}

	// 3. Загрузка списков модерации (из moderation.go)
	loadModerationLists()

	// 4. Инициализация статистики (из stats.go)
	statsManager = NewStatsManager(appStatsFilePath)
	log.Printf("✅ Статистика загружена. Сообщений: %d, Забанено: %d", statsManager.Data.TotalMessages, statsManager.Data.BannedUsers)

	// 5. Инициализация менеджера женщин (SQLite)
	// ВАЖНО: Используем women.db вместо .json
	womanManager = NewWomanManager(dbFilePath)
	log.Println("✅ База данных женщин (SQLite) подключена.")

	// 6. Настройки бота
	log.Println("🔄 Попытка подключения к Telegram API...")

	pref := tele.Settings{
		Token: config.Token,
		// ВАЖНО: Подключаем Cloudflare Worker здесь
		// Если в конфиге есть URL, используем его, иначе библиотека возьмет стандартный
		URL: config.BotAPIUrl,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
		// Добавляем свой логгер для отладки
		OnError: func(err error, c tele.Context) {
			// Этот блок будет ловить ошибки апдейтов (таймауты, разрывы связи)
			log.Printf("❌ Ошибка в Bot Poller: %v", err)
			if c != nil {
				log.Printf("   -> В чате: %v", c.Chat().ID)
			}
		},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("❌ КРИТИЧЕСКАЯ ОШИБКА при создании бота (проверьте токен или доступ к API): %v", err)
	}

	// 7. Инициализация меню (из handlers.go)
	InitMenus()

	// 8. Регистрация всех хендлеров (из handlers.go)
	RegisterHandlers(b)

	// 9. Запуск Умного Планировщика (из scheduler.go)
	// Он будет проверять настройки в БД и отправлять пост в нужное время
	safeGo("scheduler", func() { StartScheduler(b, womanManager, config.TargetChatID) })
	safeGo("housekeeping", startHousekeeping)
	if addr := os.Getenv("OPHELIA_HEALTH_ADDR"); addr != "" {
		safeGo("health-server", func() { startHealthServer(addr) })
	}

	// Информация о боте (b.Me заполняется автоматически при NewBot)
	log.Printf("✅ Соединение установлено! Бот: @%s (ID: %d)", b.Me.Username, b.Me.ID)
	if config.BotAPIUrl != "" {
		log.Printf("🌐 Работа через прокси (Cloudflare): %s", config.BotAPIUrl)
	} else {
		log.Println("🌐 Работа через стандартный api.telegram.org (может быть заблокирован в РФ)")
	}

	// =========================================================================
	// 🧹 СБРОС ОЧЕРЕДИ И ВЕБХУКА (ОЧЕНЬ ВАЖНО ПРИ СМЕНЕ СЕРВЕРА/ПРОКСИ)
	// =========================================================================
	log.Println("🧹 Сброс вебхука и удаление старых зависших сообщений...")
	// Аргумент 'true' означает drop_pending_updates=True
	// Это удалит все старые сообщения, которые накопились пока бот не работал
	if err := b.RemoveWebhook(true); err != nil {
		log.Printf("⚠️ Предупреждение: Не удалось сбросить вебхук (возможно, ошибка сети): %v", err)
	} else {
		log.Println("✅ Вебхук удален, очередь очищена. Бот готов к работе.")
	}

	fmt.Printf("🚀 Бот запущен. Target: %d. Admins: %d\n", config.TargetChatID, len(getAdmins()))

	// Запускаем бота
	safeGo("bot", func() { b.Start() })

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("⏹ Завершение работы...")
	b.Stop()
	if err := womanManager.CloseDB(); err != nil {
		log.Printf("⚠️ Ошибка закрытия БД: %v", err)
	}
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if v := os.Getenv("OPHELIA_BOT_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("OPHELIA_GIGACHAT_KEY"); v != "" {
		cfg.GoogleAPI = v
	}
	if v := os.Getenv("OPHELIA_BOT_API_URL"); v != "" {
		cfg.BotAPIUrl = v
	}
	if v := os.Getenv("OPHELIA_TARGET_CHAT_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.TargetChatID = id
		}
	}
}
