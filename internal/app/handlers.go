package app

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ==========================================
// КОНФИГУРАЦИЯ КАТЕГОРИЙ
// ==========================================

var defaultCategories = []string{
	"Точные науки и Технологии",
	"Медицина и Естественные науки",
	"Политика и Госуправление",
	"Философия и Мысль",
	"Исследования и Открытия",
	"Экономика и Бизнес",
	"Активизм и Правозащита",
	"Литература и Журналистика",
	"Искусство и Архитектура",
	"Образование и Просвещение",
}

// ==========================================
// МЕНЮ И КНОПКИ
// ==========================================

const (
	btnTextUserMe      = "Личное дело"
	btnTextUserWomen   = "Архив имен"
	btnTextUserTop     = "Доска почета"
	btnTextUserSuggest = "Внести предложение"
	btnTextUserRandom  = "Случайная запись"
	btnTextUserSelect  = "Подборка дня"
	btnTextUserEra     = "Эпохи"
	btnTextUserTheme   = "Тема недели"
	btnTextUserTags    = "Теги"
	btnTextUserBrowse  = "Навигация"
	btnTextUserFavs    = "Избранное"
	btnTextUserRec     = "Рекомендации"
	btnTextUserDaily   = "Ежедневник"
)

const (
	cbStartQuiz        = "start_quiz"
	cbShowStats        = "show_stats"
	cbAddWoman         = "add_woman"
	cbAdminInbox       = "admin_inbox"
	cbEditWomanSearch  = "edit_woman_search"
	cbAdminNoTags      = "admin_notags"
	cbBotSettings      = "bot_settings"
	cbDBMenu           = "db_menu"
	cbManageWords      = "manage_words"
	cbAdminDiag        = "admin_diag"
	cbAdminBroadcast   = "admin_broadcast"
	cbAdminAudit       = "admin_audit"
	cbAdminWhitelist   = "admin_whitelist"
	cbAdminChats       = "admin_chats"
	cbInboxApprove     = "inbox_approve"
	cbInboxReject      = "inbox_reject"
	cbAdminBackMain    = "admin_back_main"
	cbFinishSuggest    = "finish_suggest"
	cbDBBackup         = "db_backup"
	cbDBImport         = "db_import"
	cbDBVacuum         = "db_vacuum"
	cbEditMediaAdd     = "edit_media_add"
	cbEditMediaClear   = "edit_media_clear"
	cbShowAllWomenEdit = "show_all_women_edit"
	cbSettingsToggle   = "settings_toggle"
	cbSettingsSetTime  = "settings_set_time"
	cbAddWord          = "add_word"
	cbRemoveWord       = "remove_word"
	cbListWords        = "list_words"
	cbModePainting     = "mode_painting"
	cbModeQuotes       = "mode_quotes"
	cbModeDesc         = "mode_desc"
	cbRefreshStats     = "refresh_stats"
	cbThemeMore        = "theme_more"
	cbFinishWomanPhoto = "finish_woman_photo"
	cbSaveDraft        = "save_draft"
	cbCancelSuggest    = "cancel_suggest"
	cbConfirmYes       = "confirm_yes"
	cbConfirmNo        = "confirm_no"
	cbBackToEditRecord = "back_to_edit_record"
)

const (
	userStateGCInterval = 12 * time.Hour
	userStateMaxIdle    = 24 * time.Hour
)

var (
	// СОСТОЯНИЯ
	adminStates = make(map[int64]string)

	// Храним ID цели редактирования/просмотра
	adminEditTarget = make(map[int64]uint) // ID записи в БД
	adminEditField  = make(map[int64]string)

	adminStatesMu sync.Mutex

	// --- ANTI-SPAM VARIABLES ---
	userLastReq   = make(map[int64]time.Time)
	userLastReqMu sync.Mutex

	// --- USER STATE ---
	userLastShown   = make(map[int64]uint)
	userLastShownMu sync.Mutex

	quizStates   = make(map[int64]quizState)
	quizStatesMu sync.Mutex

	userLastTheme   = make(map[int64]string)
	userLastThemeMu sync.Mutex

	pendingActions   = make(map[int64]pendingAction)
	pendingActionsMu sync.Mutex

	adminActionLast   = make(map[int64]map[string]time.Time)
	adminActionLastMu sync.Mutex

	searchSuggestMu sync.Mutex
	searchSuggest   = make(map[int64]searchSuggestion)

	browseStateMu sync.Mutex
	browseStates  = make(map[int64]browseState)
	browseCacheMu sync.Mutex
	browseCaches  = make(map[int64]browseCache)

	userStateGCOnce sync.Once
)

// Маршруты callback-данных для нового иерархического UI.
const (
	cbMainMenu     = "ui_main"
	cbMainSite     = "ui_main_site"
	cbMainFun      = "ui_main_fun"
	cbMainAdmin    = "ui_main_admin"
	cbBackToMain   = "ui_back_main"
	cbSiteHome     = "ui_site_home"
	cbSiteAbout    = "ui_site_about"
	cbSiteProjects = "ui_site_projects"
	cbSiteSkills   = "ui_site_skills"
	cbSiteContacts = "ui_site_contacts"
	cbFunWoman     = "ui_fun_woman"
	cbFunStats     = "ui_fun_stats"
	cbAdminEvents  = "ui_admin_events"
	cbAdminLogs    = "ui_admin_logs"
	cbAdminCMS     = "ui_admin_cms"
)

// КОНСТАНТЫ СОСТОЯНИЙ
const (
	STATE_IDLE                = ""
	STATE_WAITING_PHOTO       = "waiting_photo"
	STATE_WAITING_ANSWER      = "waiting_answer"
	STATE_WAITING_CONTEXT     = "waiting_context"
	STATE_WAITING_ADD_WORD    = "waiting_add_word"
	STATE_WAITING_REMOVE_WORD = "waiting_remove_word"

	// Состояния БД
	STATE_WAITING_DB_IMPORT = "waiting_db_file"
	STATE_WAITING_BROADCAST = "waiting_broadcast"
	STATE_WAITING_CONFIRM   = "waiting_confirm"
	STATE_WAITING_WL_ADD    = "waiting_wl_add"
	STATE_WAITING_REJECT    = "waiting_reject_reason"

	// Состояния добавления
	STATE_WOMAN_NAME  = "woman_name"
	STATE_WOMAN_FIELD = "woman_field"
	STATE_WOMAN_YEAR  = "woman_year"
	STATE_WOMAN_INFO  = "woman_info"
	STATE_WOMAN_TAGS  = "woman_tags"
	STATE_WOMAN_MEDIA = "woman_media"

	// Состояния редактирования
	STATE_EDIT_SEARCH    = "edit_search"
	STATE_EDIT_VALUE     = "edit_value"
	STATE_EDIT_MEDIA_ADD = "edit_media_add_mode"

	// Состояния настроек
	STATE_WAITING_TIME = "waiting_schedule_time"
)

type quizState struct {
	WomanID uint
	Options []string
	Correct int
}

type pendingAction struct {
	Action   string
	TargetID uint
	KeepID   uint
	RemoveID uint
	Tag      string
	AddTags  bool
	Filters  SearchFilters
	FilePath string
}

type searchSuggestion struct {
	Tags   []string
	Fields []string
}

type browseState struct {
	YearFrom int
	YearTo   int
	Field    string
	Tag      string
}

type browseCache struct {
	Fields []string
	Tags   []string
}

// ==========================================
// ИНИЦИАЛИЗАЦИЯ
// ==========================================

func InitMenus() {
	// Меню собираются динамически функциями build*Menu().
}

func buildMainMenu(userID int64) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnSite := m.Data("Сайт", cbMainSite)
	btnFun := m.Data("Развлечения", cbMainFun)
	rows := []tele.Row{
		m.Row(btnSite, btnFun),
	}
	if isAdmin(userID) {
		btnAdmin := m.Data("Админка", cbMainAdmin)
		rows = append(rows, m.Row(btnAdmin))
	}
	m.Inline(rows...)
	return m
}

func buildSiteMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnHome := m.Data("Главная", cbSiteHome)
	btnAbout := m.Data("О себе", cbSiteAbout)
	btnProjects := m.Data("Проекты", cbSiteProjects)
	btnSkills := m.Data("Навыки", cbSiteSkills)
	btnContacts := m.Data("Контакты", cbSiteContacts)
	btnBack := m.Data("🔙 Назад", cbBackToMain)
	m.Inline(
		m.Row(btnHome, btnAbout),
		m.Row(btnProjects, btnSkills),
		m.Row(btnContacts),
		m.Row(btnBack),
	)
	return m
}

func buildFunMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnWoman := m.Data("Женщина", cbFunWoman)
	btnStats := m.Data("Статистика", cbFunStats)
	btnBack := m.Data("🔙 Назад", cbBackToMain)
	m.Inline(
		m.Row(btnWoman, btnStats),
		m.Row(btnBack),
	)
	return m
}

func buildAdminMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnLogs := m.Data("Логи", cbAdminLogs)
	btnEvents := m.Data("Мероприятия", cbAdminEvents)
	btnCMS := m.Data("Управление Сайтом", cbAdminCMS)
	btnBack := m.Data("🔙 Назад", cbBackToMain)
	m.Inline(
		m.Row(btnLogs, btnEvents),
		m.Row(btnCMS),
		m.Row(btnBack),
	)
	return m
}

func showMainInlineMenu(c tele.Context, edit bool) error {
	userID := int64(0)
	if c.Sender() != nil {
		userID = c.Sender().ID
	}
	msg := "Выберите раздел:"
	if edit {
		return tryEdit(c, msg, buildMainMenu(userID), tele.ModeHTML)
	}
	return c.Send(msg, buildMainMenu(userID), tele.ModeHTML)
}

func buildUserReplyMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnUserMe := m.Text(btnTextUserMe)
	btnUserWomen := m.Text(btnTextUserWomen)
	btnUserTop := m.Text(btnTextUserTop)
	btnUserSuggest := m.Text(btnTextUserSuggest)
	btnUserRandom := m.Text(btnTextUserRandom)
	btnUserSelect := m.Text(btnTextUserSelect)
	btnUserEra := m.Text(btnTextUserEra)
	btnUserTheme := m.Text(btnTextUserTheme)
	btnUserTags := m.Text(btnTextUserTags)
	btnUserBrowse := m.Text(btnTextUserBrowse)
	btnUserFavs := m.Text(btnTextUserFavs)
	btnUserRec := m.Text(btnTextUserRec)
	btnUserDaily := m.Text(btnTextUserDaily)
	m.Reply(
		m.Row(btnUserWomen, btnUserSuggest),
		m.Row(btnUserMe, btnUserTop),
		m.Row(btnUserRandom, btnUserSelect),
		m.Row(btnUserTheme, btnUserEra),
		m.Row(btnUserTags, btnUserBrowse),
		m.Row(btnUserFavs, btnUserRec),
		m.Row(btnUserDaily),
	)
	return m
}

func buildRulesMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnRules := m.URL("Ознакомиться с уставом", "https://telegra.ph/Pravila-chata-Ophelia-la-glaneuse-12-24")
	m.Inline(m.Row(btnRules))
	return m
}

func buildInboxTitle(pendingCount int64) string {
	if pendingCount > 0 {
		return fmt.Sprintf("Корреспонденция (%d)", pendingCount)
	}
	return "Корреспонденция"
}

func buildAdminPanelMenu(pendingCount int64) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnInlineStart := m.Data("Начать испытание", cbStartQuiz)
	btnAddWoman := m.Data("Новая запись", cbAddWoman)
	btnInbox := m.Data(buildInboxTitle(pendingCount), cbAdminInbox)
	btnEditWoman := m.Data("Реестр / Поиск", cbEditWomanSearch)
	btnNoTags := m.Data("Без тегов", cbAdminNoTags)
	btnDatabase := m.Data("Хранилище", cbDBMenu)
	btnSettings := m.Data("Хронограф", cbBotSettings)
	btnManageWords := m.Data("Цензура", cbManageWords)
	btnInlineStats := m.Data("Общая статистика", cbShowStats)
	btnInlineDiag := m.Data("Диагностика", cbAdminDiag)
	btnInlineAudit := m.Data("Аудит", cbAdminAudit)
	btnBroadcast := m.Data("Созвать всех", cbAdminBroadcast)
	btnWhitelist := m.Data("Белый список", cbAdminWhitelist)
	btnChats := m.Data("Чаты", cbAdminChats)
	m.Inline(
		m.Row(btnInlineStart),
		m.Row(btnAddWoman, btnInbox),
		m.Row(btnEditWoman, btnNoTags),
		m.Row(btnDatabase, btnSettings),
		m.Row(btnManageWords, btnInlineStats),
		m.Row(btnInlineDiag, btnInlineAudit),
		m.Row(btnBroadcast, btnWhitelist),
		m.Row(btnChats),
	)
	return m
}

func buildModPanelMenu(pendingCount int64) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnInlineStats := m.Data("Общая статистика", cbShowStats)
	btnInlineDiag := m.Data("Диагностика", cbAdminDiag)
	btnInlineAudit := m.Data("Аудит", cbAdminAudit)
	btnInbox := m.Data(buildInboxTitle(pendingCount), cbAdminInbox)
	btnEditWoman := m.Data("Реестр / Поиск", cbEditWomanSearch)
	btnNoTags := m.Data("Без тегов", cbAdminNoTags)
	m.Inline(
		m.Row(btnInlineStats, btnInlineDiag),
		m.Row(btnInlineAudit, btnInbox),
		m.Row(btnEditWoman, btnNoTags),
	)
	return m
}

func buildStaffPanelMenu(userID int64) *tele.ReplyMarkup {
	pending := womanManager.CountPending()
	if isAdmin(userID) {
		return buildAdminPanelMenu(pending)
	}
	return buildModPanelMenu(pending)
}

func buildStaffPanelMenuForContext(c tele.Context) *tele.ReplyMarkup {
	if c == nil || c.Sender() == nil {
		return buildAdminPanelMenu(womanManager.CountPending())
	}
	return buildStaffPanelMenu(c.Sender().ID)
}

func buildInboxMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnInboxApprove := m.Data("Утвердить", cbInboxApprove)
	btnInboxReject := m.Data("Отвергнуть", cbInboxReject)
	btnBackToAdmin := m.Data("Вернуться в меню", cbAdminBackMain)
	m.Inline(
		m.Row(btnInboxApprove, btnInboxReject),
		m.Row(btnBackToAdmin),
	)
	return m
}

func buildFinishSuggestMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnFinishSuggest := m.Data("Направить на рассмотрение", cbFinishSuggest)
	m.Inline(m.Row(btnFinishSuggest))
	return m
}

func buildDBMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnBackup := m.Data("Экспорт (Backup)", cbDBBackup)
	btnImport := m.Data("Импорт (Restore)", cbDBImport)
	btnVacuum := m.Data("Оптимизация (Vacuum)", cbDBVacuum)
	btnBackFromDB := m.Data("Назад", cbAdminBackMain)
	m.Inline(
		m.Row(btnBackup),
		m.Row(btnImport, btnVacuum),
		m.Row(btnBackFromDB),
	)
	return m
}

func buildEditMediaMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnEditMediaAdd := m.Data("Добавить изображение", cbEditMediaAdd)
	btnEditMediaClear := m.Data("Очистить галерею", cbEditMediaClear)
	btnBackToEdit := m.Data("Назад к записи", cbBackToEditRecord)
	m.Inline(
		m.Row(btnEditMediaAdd),
		m.Row(btnEditMediaClear),
		m.Row(btnBackToEdit),
	)
	return m
}

func buildSettingsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnToggleSchedule := m.Data("Вкл / Выкл", cbSettingsToggle)
	btnSetTime := m.Data("Установить время", cbSettingsSetTime)
	btnBackFromSettings := m.Data("Назад", cbAdminBackMain)
	m.Inline(
		m.Row(btnToggleSchedule),
		m.Row(btnSetTime),
		m.Row(btnBackFromSettings),
	)
	return m
}

func buildWordsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnAddWord := m.Data("Добавить", cbAddWord)
	btnRemoveWord := m.Data("Удалить", cbRemoveWord)
	btnListWords := m.Data("Просмотр списка", cbListWords)
	btnBackFromWords := m.Data("Назад", cbAdminBackMain)
	m.Inline(
		m.Row(btnAddWord, btnRemoveWord),
		m.Row(btnListWords),
		m.Row(btnBackFromWords),
	)
	return m
}

func buildModesMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnModePainting := m.Data("Живопись", cbModePainting)
	btnModeQuotes := m.Data("Цитаты", cbModeQuotes)
	btnModeDesc := m.Data("Биография", cbModeDesc)
	btnBackToMain := m.Data("Назад", cbAdminBackMain)
	m.Inline(
		m.Row(btnModePainting),
		m.Row(btnModeQuotes, btnModeDesc),
		m.Row(btnBackToMain),
	)
	return m
}

func buildStatsMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnRefreshStats := m.Data("Обновить данные", cbRefreshStats)
	btnBackFromStat := m.Data("Назад", cbAdminBackMain)
	m.Inline(
		m.Row(btnRefreshStats),
		m.Row(btnBackFromStat),
	)
	return m
}

func buildThemeMoreMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnThemeMore := m.Data("Еще по теме", cbThemeMore)
	m.Inline(m.Row(btnThemeMore))
	return m
}

func buildFinishPhotoMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnFinishPhoto := m.Data("Зафиксировать в летописи", cbFinishWomanPhoto)
	btnSaveDraft := m.Data("Сохранить черновик", cbSaveDraft)
	m.Inline(
		m.Row(btnFinishPhoto),
		m.Row(btnSaveDraft),
	)
	return m
}

func buildCancelEditMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnCancelEdit := m.Data("Прервать", cbAdminBackMain)
	m.Inline(m.Row(btnCancelEdit))
	return m
}

func buildCancelSuggestMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnCancelSuggest := m.Data("Отозвать", cbCancelSuggest)
	m.Inline(m.Row(btnCancelSuggest))
	return m
}

func buildConfirmMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnConfirmYes := m.Data("Подтвердить", cbConfirmYes)
	btnConfirmNo := m.Data("Отмена", cbConfirmNo)
	m.Inline(m.Row(btnConfirmYes, btnConfirmNo))
	return m
}

func RegisterHandlers(b *tele.Bot) {
	startUserStateCollector()

	// Основные Команды
	b.Handle("/start", HandleStart)
	b.Handle("/help", HandleHelp)
	b.Handle("/admin", HandleAdminPanel)
	b.Handle("/sendinfo", HandleSendInfo) // Рассылка
	b.Handle("/status", HandleStatus)
	b.Handle("/reload", HandleReload)
	b.Handle("/verify", HandleVerify)
	b.Handle("/unverify", HandleUnverify)
	b.Handle("/audit", HandleAudit)
	b.Handle("/broadcasts", HandleBroadcasts)
	b.Handle("/export", HandleExport)
	b.Handle("/merge", HandleMerge)
	b.Handle("/tagadd", HandleTagAdd)
	b.Handle("/tagremove", HandleTagRemove)
	b.Handle("/whitelist", HandleWhitelist)
	b.Handle("/wladd", HandleWhitelistAdd)
	b.Handle("/wldel", HandleWhitelistDel)
	b.Handle("/whitelist_add", HandleWhitelistAdd)
	b.Handle("/whitelist_del", HandleWhitelistDel)
	b.Handle("/chats", HandleChats)
	b.Handle("/coladd", HandleCollectionAdd)
	b.Handle("/coldel", HandleCollectionDel)
	b.Handle("/collist", HandleCollectionList)
	b.Handle("/colpub", HandleCollectionPublish)
	b.Handle("/colunpub", HandleCollectionUnpublish)
	b.Handle("/mediacheck", HandleMediaCheck)
	b.Handle("/history", HandleHistory)
	b.Handle("/tagsuggest", HandleTagSuggest)
	b.Handle("/modadd", HandleModAdd)
	b.Handle("/moddel", HandleModDel)
	b.Handle("/modlist", HandleModList)
	b.Handle("/modlog", HandleModLog)
	b.Handle("/dups", HandleDuplicates)
	b.Handle("/quality", HandleQuality)
	b.Handle("/topcards", HandleTopCards)
	b.Handle("/theme_on", HandleThemeOn)
	b.Handle("/theme_off", HandleThemeOff)
	b.Handle("/theme_time", HandleThemeTime)
	b.Handle("/theme_day", HandleThemeDay)
	b.Handle("/health_on", HandleHealthOn)
	b.Handle("/health_off", HandleHealthOff)
	b.Handle("/health_time", HandleHealthTime)
	b.Handle("/report_on", HandleReportOn)
	b.Handle("/report_off", HandleReportOff)
	b.Handle("/report_time", HandleReportTime)
	b.Handle("/report_day", HandleReportDay)
	b.Handle("/inbox", HandleInbox)
	b.Handle("/cms_post", HandleCMSPostCommand)
	b.Handle("/event_manage", HandleCMSEventManageCommand)
	b.Handle("/cms_event_add", HandleCMSEventAddCommand)
	b.Handle("/cms_post_del", HandleCMSPostDelCommand)
	b.Handle("/cms_site", HandleCMSSiteCommand)

	// Команды пользователя
	b.Handle("/me", HandleMe)
	b.Handle("/women", HandleUserWoman)
	b.Handle("/top", HandleTop)
	b.Handle("/id", HandleID)
	b.Handle("/suggest", HandleStartSuggest)
	b.Handle("/random", HandleRandomWoman)
	b.Handle("/selection", HandleSelection)
	b.Handle("/era", HandleEraMenu)
	b.Handle("/century", HandleCenturyMenu)
	b.Handle("/theme", HandleTheme)
	b.Handle("/search", HandleSearch)
	b.Handle("/tags", HandleTagsMenu)
	b.Handle("/browse", HandleBrowse)
	b.Handle("/fav", HandleFavorites)
	b.Handle("/rec", HandleRecommendations)
	b.Handle("/daily_on", HandleDailyOn)
	b.Handle("/daily_off", HandleDailyOff)
	b.Handle("/daily_time", HandleDailyTime)
	b.Handle("/collections", HandleCollections)

	// Кнопки клавиатуры
	b.Handle(btnTextUserMe, HandleMe)
	b.Handle(btnTextUserWomen, HandleUserWoman)
	b.Handle(btnTextUserTop, HandleTop)
	b.Handle(btnTextUserSuggest, HandleStartSuggest)
	b.Handle(btnTextUserRandom, HandleRandomWoman)
	b.Handle(btnTextUserSelect, HandleSelection)
	b.Handle(btnTextUserEra, HandleEraMenu)
	b.Handle(btnTextUserTheme, HandleTheme)
	b.Handle(btnTextUserTags, HandleTagsMenu)
	b.Handle(btnTextUserBrowse, HandleBrowse)
	b.Handle(btnTextUserFavs, HandleFavorites)
	b.Handle(btnTextUserRec, HandleRecommendations)
	b.Handle(btnTextUserDaily, HandleDailyStatus)

	b.Handle("/stopgame", HandleStopGame)

	// --- КАПЧА И CALLBACK ---
	b.Handle(tele.OnCallback, func(c tele.Context) error {
		// Всегда подтверждаем callback, чтобы убрать "часики" на кнопке.
		defer func() {
			_ = c.Respond()
		}()

		data := strings.TrimSpace(c.Callback().Data)
		userID := c.Sender().ID

		// Проверка капчи
		if strings.HasPrefix(data, "captcha_") {
			parts := strings.Split(data, "_")
			if len(parts) != 2 {
				return c.Respond()
			}

			if parts[1] == "correct" {
				womanManager.SetUserVerified(userID)
				c.Delete()
				c.Respond(&tele.CallbackResponse{Text: "Доступ разрешен."})
				return HandleStart(c) // Запускаем нормальный старт
			} else {
				c.Respond(&tele.CallbackResponse{Text: "Ошибка. Попробуйте снова."})
				c.Delete()
				return sendCaptcha(c)
			}
		}
		// Передаем остальные колбэки в процессор
		return processCallback(c)
	})

	registerCallback := func(unique string, handler tele.HandlerFunc) {
		btn := tele.Btn{Unique: unique}
		b.Handle(&btn, handler)
	}

	registerCallback(cbFinishWomanPhoto, func(c tele.Context) error {
		c.Respond()
		if getAdminState(c.Sender().ID) != STATE_WOMAN_MEDIA {
			return c.Send("Ошибка: нарушение последовательности.")
		}
		err := womanManager.SaveDraft(c.Sender().ID, true)
		if err != nil {
			return c.Send("Ошибка записи: " + err.Error())
		}
		setAdminState(c.Sender().ID, STATE_IDLE)
		c.Delete()
		return c.Send("Запись успешно внесена в летопись.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	})

	registerCallback(cbSaveDraft, func(c tele.Context) error {
		c.Respond()
		if getAdminState(c.Sender().ID) != STATE_WOMAN_MEDIA {
			return c.Send("Ошибка: нарушение последовательности.")
		}
		err := womanManager.SaveDraft(c.Sender().ID, false)
		if err != nil {
			return c.Send("Ошибка записи: " + err.Error())
		}
		setAdminState(c.Sender().ID, STATE_IDLE)
		c.Delete()
		return c.Send("Черновик сохранен.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	})

	registerCallback(cbFinishSuggest, func(c tele.Context) error {
		c.Respond()
		if getAdminState(c.Sender().ID) != STATE_WOMAN_MEDIA {
			return c.Send("Произошла ошибка.")
		}
		err := womanManager.SaveDraft(c.Sender().ID, false)
		if err != nil {
			return c.Send("Ошибка сохранения: " + err.Error())
		}
		setAdminState(c.Sender().ID, STATE_IDLE)
		c.Delete()
		return c.Send("Благодарю. Ваше предложение направлено на рассмотрение.", tele.ModeHTML)
	})

	registerCallback(cbCancelSuggest, func(c tele.Context) error {
		setAdminState(c.Sender().ID, STATE_IDLE)
		c.Delete()
		return c.Send("Предложение отозвано.")
	})

	// Остальные хендлеры (меню и кнопки)
	registerCallback(cbStartQuiz, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		return tryEdit(c, "Выбор режима испытания", buildModesMenu(), tele.ModeHTML)
	})
	registerCallback(cbAdminBackMain, HandleBackToMain)
	registerCallback(cbShowStats, HandleShowStats)
	registerCallback(cbRefreshStats, HandleShowStats)
	registerCallback(cbManageWords, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		return tryEdit(c, "Редактирование запретов", buildWordsMenu(), tele.ModeHTML)
	})

	registerCallback(cbAddWord, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		setAdminState(c.Sender().ID, STATE_WAITING_ADD_WORD)
		return tryEdit(c, "Введите слово для запрета:", tele.ModeHTML)
	})
	registerCallback(cbRemoveWord, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		setAdminState(c.Sender().ID, STATE_WAITING_REMOVE_WORD)
		return tryEdit(c, "Введите слово для амнистии:", tele.ModeHTML)
	})
	registerCallback(cbListWords, HandleListWords)

	registerCallback(cbAddWoman, func(c tele.Context) error {
		if !isStaff(c.Sender().ID) {
			return nil
		}
		womanManager.StartAdding(c.Sender().ID)
		_ = womanManager.WithDraft(c.Sender().ID, func(d *Woman) error {
			d.SuggestedBy = 0
			return nil
		})
		setAdminState(c.Sender().ID, STATE_WOMAN_NAME)
		c.Delete()
		return c.Send("Создание новой записи.\nВведите Имя и Фамилию:", tele.ModeHTML)
	})

	registerCallback(cbEditWomanSearch, func(c tele.Context) error {
		if !isStaff(c.Sender().ID) {
			return nil
		}
		setAdminState(c.Sender().ID, STATE_EDIT_SEARCH)
		return tryEdit(c, "Введите имя для поиска в реестре:", buildCancelEditMenu(), tele.ModeHTML)
	})

	registerCallback(cbModePainting, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		gameManager.SetupGameMode("painting")
		setAdminState(c.Sender().ID, STATE_WAITING_PHOTO)
		return tryEdit(c, "Предоставьте полотно для анализа.", tele.ModeHTML)
	})
	registerCallback(cbModeQuotes, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		gameManager.SetupGameMode("mode_quotes")
		setAdminState(c.Sender().ID, STATE_WAITING_ANSWER)
		return tryEdit(c, "Введите верный ответ:", tele.ModeHTML)
	})
	registerCallback(cbModeDesc, func(c tele.Context) error {
		if !isAdmin(c.Sender().ID) {
			return nil
		}
		gameManager.SetupGameMode("mode_desc")
		setAdminState(c.Sender().ID, STATE_WAITING_ANSWER)
		return tryEdit(c, "Введите верный ответ:", tele.ModeHTML)
	})

	b.Handle(tele.OnPhoto, HandlePhoto)
	b.Handle(tele.OnDocument, HandleDocument)
	b.Handle(tele.OnText, HandleText)
	b.Handle(tele.OnEdited, HandleText)
	b.Handle(tele.OnSticker, func(c tele.Context) error { return nil })

	// ВАЖНО: Middleware подключаем после всех хендлеров
	b.Use(RecoverMiddleware())
	b.Use(Middleware())

	b.Handle(tele.OnUserJoined, HandleUserJoin)
	b.Handle(tele.OnUserLeft, func(c tele.Context) error { return c.Delete() })
}

// ==========================================
// ЛОГИКА CALLBACK (ВЫНЕСЕНА)
// ==========================================

func processCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	userID := c.Sender().ID

	if cmsService != nil {
		if handled, err := cmsService.HandleBotCMSCallback(c, data); handled {
			return err
		}
	}

	// Новый callback-router для многоуровневого UI (Главное -> Сайт/Развлечения/Админ).
	// Здесь делаем только редактирование текущего сообщения, чтобы не спамить чат.
	switch data {
	case cbMainMenu, cbBackToMain:
		resetCMSAdminState(userID)
		return showMainInlineMenu(c, true)
	case cbMainSite:
		return tryEdit(c, "Раздел сайта. Выберите страницу:", buildSiteMenu(), tele.ModeHTML)
	case cbMainFun:
		return tryEdit(c, "Раздел развлечений. Выберите действие:", buildFunMenu(), tele.ModeHTML)
	case cbMainAdmin:
		if !isAdmin(userID) {
			return tryEdit(c, "Доступ к админке закрыт.", buildMainMenu(userID), tele.ModeHTML)
		}
		return tryEdit(c, "Админка. Выберите раздел:", buildAdminMenu(), tele.ModeHTML)
	case cbAdminCMS:
		if !isAdmin(userID) {
			return tryEdit(c, "Недостаточно прав.", buildMainMenu(userID), tele.ModeHTML)
		}
		if cmsService == nil {
			return tryEdit(c, "CMS-репозиторий не инициализирован.", buildAdminMenu(), tele.ModeHTML)
		}
		return cmsService.HandleBotSiteAdminMenu(c)
	case cbSiteHome:
		return tryEdit(c, "Главная: новости и события доступны на сайте.", buildSiteMenu(), tele.ModeHTML)
	case cbSiteAbout:
		return tryEdit(c, "О себе: Офелия ведет архив биографий и образовательные подборки.", buildSiteMenu(), tele.ModeHTML)
	case cbSiteProjects:
		return tryEdit(c, "Проекты: летопись, викторины, подборки, CMS и события.", buildSiteMenu(), tele.ModeHTML)
	case cbSiteSkills:
		return tryEdit(c, "Навыки: поиск по тегам, эпохам, векам и персональные рекомендации.", buildSiteMenu(), tele.ModeHTML)
	case cbSiteContacts:
		return tryEdit(c, "Контакты: используйте /help и команды бота для связи и модерации.", buildSiteMenu(), tele.ModeHTML)
	case cbFunWoman:
		w := womanManager.GetRandomWoman()
		if w == nil {
			return tryEdit(c, "Архив пока пуст.", buildFunMenu(), tele.ModeHTML)
		}
		preview := fmt.Sprintf("👤 <b>%s</b>\n📚 %s\n🕰 %s\n\nИспользуйте /random для полной карточки с медиа.", html.EscapeString(w.Name), html.EscapeString(w.Field), html.EscapeString(w.Year))
		return tryEdit(c, preview, buildFunMenu(), tele.ModeHTML)
	case cbFunStats:
		if c.Sender() == nil {
			return nil
		}
		return tryEdit(c, buildUserStatsText(c.Sender().ID), buildFunMenu(), tele.ModeHTML)
	case cbAdminEvents:
		if !isAdmin(userID) {
			return tryEdit(c, "Недостаточно прав.", buildMainMenu(userID), tele.ModeHTML)
		}
		return tryEdit(c, "Управление мероприятиями:\n/event_manage — список и участники\n/cms_event_add — добавить событие", buildAdminMenu(), tele.ModeHTML)
	case cbAdminLogs:
		if !isAdmin(userID) {
			return tryEdit(c, "Недостаточно прав.", buildMainMenu(userID), tele.ModeHTML)
		}
		return tryEdit(c, "Логи и диагностика:\n/status, /audit, /history, /broadcasts", buildAdminMenu(), tele.ModeHTML)
	}

	// --- ВЫБОР СФЕРЫ (КАТЕГОРИИ) ---
	if data == cbConfirmYes {
		return executePendingAction(c)
	}
	if data == cbConfirmNo {
		if act, ok := getPendingAction(userID); ok {
			if act.Action == cbDBImport && act.FilePath != "" {
				_ = os.Remove(act.FilePath)
			}
		}
		clearPendingAction(userID)
		setAdminState(userID, STATE_IDLE)
		return tryEdit(c, "Действие отменено.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}

	if strings.HasPrefix(data, "set_cat_") {
		idxStr := strings.TrimPrefix(data, "set_cat_")
		idx, _ := strconv.Atoi(idxStr)
		state := getAdminState(userID)
		if state != STATE_WOMAN_FIELD {
			return c.Respond(&tele.CallbackResponse{Text: "Действие недоступно"})
		}
		if idx >= 0 && idx < len(defaultCategories) {
			selectedCategory := defaultCategories[idx]
			if err := womanManager.WithDraft(userID, func(d *Woman) error {
				d.Field = selectedCategory
				return nil
			}); err == nil {
				setAdminState(userID, STATE_WOMAN_YEAR)
				menuCancel := buildCancelEditMenu()
				if !isAdmin(userID) {
					menuCancel = buildCancelSuggestMenu()
				}
				return tryEdit(c, fmt.Sprintf("Выбрана сфера: <b>%s</b>\n\nТеперь введите годы жизни:", selectedCategory), menuCancel, tele.ModeHTML)
			}
		}
		return c.Respond()
	}

	// --- INBOX ---
	if data == cbAdminInbox {
		pending := womanManager.GetPendingSuggestions()
		if len(pending) == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "Входящих сообщений нет."})
		}
		w := pending[0]
		adminStatesMu.Lock()
		adminEditTarget[userID] = w.ID
		adminStatesMu.Unlock()
		womanManager.SendWomanCard(c.Bot(), c.Chat(), &w)
		return tryEdit(c, fmt.Sprintf("Заявка от ID: %d\nВ очереди: %d", w.SuggestedBy, len(pending)), buildInboxMenu(), tele.ModeHTML)
	}
	if data == cbInboxApprove {
		adminStatesMu.Lock()
		id, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			return tryEdit(c, "Ошибка идентификатора.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		err := womanManager.ApproveWoman(id)
		if err != nil {
			return tryEdit(c, "Ошибка: "+err.Error(), buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		logModAction(userID, "approve", fmt.Sprintf("%d", id), "")
		c.Respond(&tele.CallbackResponse{Text: "Утверждено."})
		return tryEdit(c, "Запись утверждена. Проверьте корреспонденцию.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
	if data == cbInboxReject {
		adminStatesMu.Lock()
		_, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			return tryEdit(c, "Ошибка идентификатора.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		setAdminState(userID, STATE_WAITING_REJECT)
		return tryEdit(c, "Укажите причину отказа (или '-' без причины):", buildCancelEditMenu(), tele.ModeHTML)
	}
	if data == cbAdminBroadcast {
		setAdminState(userID, STATE_WAITING_BROADCAST)
		return tryEdit(c, "Введите текст воззвания. Оно будет отправлено всем известным чатам:", buildCancelEditMenu(), tele.ModeHTML)
	}
	if data == cbManageWords {
		if !isAdmin(userID) {
			return tryEdit(c, "Недостаточно прав.", buildMainMenu(userID), tele.ModeHTML)
		}
		return tryEdit(c, "Редактирование запретов", buildWordsMenu(), tele.ModeHTML)
	}
	if data == cbAdminWhitelist {
		if !hasPermission(userID, PermWhitelist) {
			return c.Respond()
		}
		return sendWhitelistPage(c, 0, true)
	}
	if strings.HasPrefix(data, "wl_page_") {
		pstr := strings.TrimPrefix(data, "wl_page_")
		p, _ := strconv.Atoi(pstr)
		if p < 0 {
			p = 0
		}
		return sendWhitelistPage(c, p, true)
	}
	if strings.HasPrefix(data, "wl_del_") {
		idStr := strings.TrimPrefix(data, "wl_del_")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if id == 0 {
			return c.Respond()
		}
		if !hasPermission(userID, PermWhitelist) {
			return c.Respond()
		}
		if removeWhitelist(id) {
			_ = saveWhitelist()
			logModAction(userID, "whitelist_remove", fmt.Sprintf("%d", id), "")
		}
		return sendWhitelistPage(c, 0, true)
	}
	if data == "wl_add" {
		if !hasPermission(userID, PermWhitelist) {
			return c.Respond()
		}
		setAdminState(userID, STATE_WAITING_WL_ADD)
		return tryEdit(c, "Введите ID чата или пользователя для белого списка:", buildCancelEditMenu(), tele.ModeHTML)
	}
	if data == cbAdminChats {
		if !hasPermission(userID, PermViewChats) {
			return c.Respond()
		}
		return sendChatsPage(c, 0, true)
	}
	if strings.HasPrefix(data, "chats_page_") {
		pstr := strings.TrimPrefix(data, "chats_page_")
		p, _ := strconv.Atoi(pstr)
		if p < 0 {
			p = 0
		}
		return sendChatsPage(c, p, true)
	}
	if data == cbAdminNoTags {
		return sendNoTagsPage(c, 0, true)
	}
	if strings.HasPrefix(data, "admin_notags_page_") {
		pstr := strings.TrimPrefix(data, "admin_notags_page_")
		p, _ := strconv.Atoi(pstr)
		if p < 0 {
			p = 0
		}
		return sendNoTagsPage(c, p, true)
	}

	// --- DB & SETTINGS ---
	if data == cbAdminDiag {
		c.Respond()
		return sendStatus(c, true)
	}
	if data == cbAdminAudit {
		c.Respond()
		return sendAudit(c, true)
	}
	if data == cbDBMenu {
		return tryEdit(c, "Управление Хранилищем Знаний.", buildDBMenu(), tele.ModeHTML)
	}
	if data == cbDBBackup {
		if !isAdmin(userID) {
			return c.Respond()
		}
		safeGo("manual-backup", func() { PerformBackup(c.Bot(), womanManager) })
		return c.Respond(&tele.CallbackResponse{Text: "Архив отправлен."})
	}
	if data == cbDBVacuum {
		if !isAdmin(userID) {
			return c.Respond()
		}
		safeGo("db-vacuum", func() {
			if err := womanManager.Vacuum(); err != nil {
				log.Printf("⚠️ Ошибка Vacuum: %v", err)
			}
		})
		return c.Respond(&tele.CallbackResponse{Text: "Оптимизация завершена."})
	}
	if data == cbDBImport {
		if !isAdmin(userID) {
			return c.Respond()
		}
		setAdminState(userID, STATE_WAITING_DB_IMPORT)
		return tryEdit(c, "Режим импорта.\nПредоставьте файл .db", buildCancelEditMenu(), tele.ModeHTML)
	}

	if data == cbBotSettings {
		return sendSettingsMenu(c)
	}
	if data == cbSettingsToggle {
		if !isAdmin(userID) {
			return c.Respond()
		}
		s, err := womanManager.GetSettings()
		if err != nil {
			log.Printf("⚠️ Ошибка чтения настроек: %v", err)
			return tryEdit(c, "Ошибка чтения настроек.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		s.IsActive = !s.IsActive
		if err := womanManager.UpdateSettings(s); err != nil {
			log.Printf("⚠️ Ошибка обновления настроек: %v", err)
		}
		return sendSettingsMenu(c)
	}
	if data == cbSettingsSetTime {
		if !isAdmin(userID) {
			return c.Respond()
		}
		setAdminState(userID, STATE_WAITING_TIME)
		return tryEdit(c, "Укажите час и минуту (09:00):", buildCancelEditMenu(), tele.ModeHTML)
	}

	// --- MENU ERA ---
	if data == "menu_back" {
		return c.Delete()
	}
	if data == "menu_eras" {
		return sendErasMenu(c, true)
	}
	if data == "menu_centuries" {
		return sendCenturiesMenu(c, true)
	}
	if strings.HasPrefix(data, "tag_page_") {
		pstr := strings.TrimPrefix(data, "tag_page_")
		p, _ := strconv.Atoi(pstr)
		return sendTagsPage(c, p, true)
	}
	if strings.HasPrefix(data, "tag_pick_") {
		tag := strings.TrimPrefix(data, "tag_pick_")
		return handleTagPick(c, tag, false)
	}
	if strings.HasPrefix(data, "tag_more_") {
		tag := strings.TrimPrefix(data, "tag_more_")
		return handleTagPick(c, tag, true)
	}
	if strings.HasPrefix(data, "fav_add_") {
		if c.Sender() == nil {
			return c.Respond()
		}
		idStr := strings.TrimPrefix(data, "fav_add_")
		id, _ := strconv.Atoi(idStr)
		if id > 0 {
			if err := womanManager.AddFavorite(c.Sender().ID, uint(id)); err != nil {
				log.Printf("⚠️ Не удалось добавить в избранное: %v", err)
			}
		}
		return c.Respond(&tele.CallbackResponse{Text: "Добавлено в избранное."})
	}
	if strings.HasPrefix(data, "fav_remove_") {
		if c.Sender() == nil {
			return c.Respond()
		}
		idStr := strings.TrimPrefix(data, "fav_remove_")
		id, _ := strconv.Atoi(idStr)
		if id > 0 {
			_ = womanManager.RemoveFavorite(c.Sender().ID, uint(id))
		}
		return c.Respond(&tele.CallbackResponse{Text: "Удалено."})
	}
	if strings.HasPrefix(data, "fav_page_") {
		if c.Sender() == nil {
			return c.Respond()
		}
		pstr := strings.TrimPrefix(data, "fav_page_")
		p, _ := strconv.Atoi(pstr)
		return sendFavoritesPage(c, c.Sender().ID, p, true)
	}
	if strings.HasPrefix(data, "rel_") {
		idStr := strings.TrimPrefix(data, "rel_")
		id, _ := strconv.Atoi(idStr)
		w, err := womanManager.GetWomanByID(uint(id))
		if err != nil || w == nil {
			return c.Respond()
		}
		items := womanManager.GetRelatedWomen(w, 3)
		if len(items) == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "Похожие не найдены."})
		}
		if c.Chat() == nil {
			return c.Respond()
		}
		for i, x := range items {
			_ = sendCardToUser(c, &x, i == len(items)-1)
			time.Sleep(120 * time.Millisecond)
		}
		return nil
	}
	if strings.HasPrefix(data, "quiz_") {
		if c.Sender() == nil || c.Chat() == nil {
			return c.Respond()
		}
		idStr := strings.TrimPrefix(data, "quiz_")
		id, _ := strconv.Atoi(idStr)
		return startQuiz(c, uint(id))
	}
	if strings.HasPrefix(data, "quiz_pick_") {
		if c.Sender() == nil {
			return c.Respond()
		}
		idxStr := strings.TrimPrefix(data, "quiz_pick_")
		idx, _ := strconv.Atoi(idxStr)
		return handleQuizPick(c, idx)
	}
	if strings.HasPrefix(data, "era_pick_") {
		c.Respond()
		return handleEraPick(c, strings.TrimPrefix(data, "era_pick_"))
	}
	if strings.HasPrefix(data, "era_page_") {
		parts := strings.Split(data, "_")
		if len(parts) >= 4 {
			code := parts[2]
			page, _ := strconv.Atoi(parts[3])
			return sendEraPage(c, code, page, true)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "era_random_") {
		code := strings.TrimPrefix(data, "era_random_")
		return handleEraRandom(c, code)
	}
	if strings.HasPrefix(data, "century_pick_") {
		c.Respond()
		centStr := strings.TrimPrefix(data, "century_pick_")
		cent, _ := strconv.Atoi(centStr)
		return handleCenturyPick(c, cent)
	}
	if strings.HasPrefix(data, "century_page_") {
		parts := strings.Split(data, "_")
		if len(parts) >= 4 {
			cent, _ := strconv.Atoi(parts[2])
			page, _ := strconv.Atoi(parts[3])
			return sendCenturyPage(c, cent, page, true)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "century_random_") {
		centStr := strings.TrimPrefix(data, "century_random_")
		cent, _ := strconv.Atoi(centStr)
		return handleCenturyRandom(c, cent)
	}

	// --- USER SEARCH ---
	if strings.HasPrefix(data, "user_show_") {
		idStr := strings.TrimPrefix(data, "user_show_")
		id, _ := strconv.Atoi(idStr)
		w, err := womanManager.GetWomanByID(uint(id))
		if err != nil || w == nil || !w.IsPublished {
			return c.Respond(&tele.CallbackResponse{Text: "Запись не найдена."})
		}
		if c.Chat() == nil {
			return c.Respond()
		}
		c.Delete()
		return sendCardToUser(c, w, true)
	}

	// --- EDITING ---
	if data == cbShowAllWomenEdit {
		results := womanManager.SearchWomen("")
		if len(results) == 0 {
			return tryEdit(c, "Реестр пуст.", buildCancelEditMenu(), tele.ModeHTML)
		}
		resultsMenu := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, w := range results {
			btn := resultsMenu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("select_edit_%d", w.ID))
			rows = append(rows, resultsMenu.Row(btn))
		}
		rows = append(rows, resultsMenu.Row(resultsMenu.Data("Прервать", cbAdminBackMain)))
		resultsMenu.Inline(rows...)
		return tryEdit(c, "Выберите запись для правки:", resultsMenu, tele.ModeHTML)
	}
	if strings.HasPrefix(data, "select_edit_") {
		idStr := strings.TrimPrefix(data, "select_edit_")
		id, _ := strconv.Atoi(idStr)
		w, err := womanManager.GetWomanByID(uint(id))
		if err != nil {
			return tryEdit(c, "Запись не обнаружена.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		adminStatesMu.Lock()
		adminEditTarget[userID] = w.ID
		adminStatesMu.Unlock()
		return sendEditMenu(c, w)
	}

	// --- MEDIA EDIT ---
	if data == "do_edit_media" {
		adminStatesMu.Lock()
		id, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			return tryEdit(c, "Ошибка доступа.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		w, err := womanManager.GetWomanByID(id)
		if err != nil || w == nil {
			return tryEdit(c, "Запись не обнаружена.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		return tryEdit(c, fmt.Sprintf("Галерея: %s (Файлов: %d)", w.Name, len(w.MediaIDs)), buildEditMediaMenu(), tele.ModeHTML)
	}
	if data == cbEditMediaClear {
		adminStatesMu.Lock()
		id, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			return c.Respond()
		}
		w, err := womanManager.GetWomanByID(id)
		if err != nil || w == nil {
			return tryEdit(c, "Запись не обнаружена.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		w.MediaIDs = []string{}
		if err := womanManager.UpdateWoman(w); err != nil {
			log.Printf("⚠️ Ошибка очистки галереи: %v", err)
			return tryEdit(c, "Ошибка очистки галереи.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		c.Respond(&tele.CallbackResponse{Text: "Галерея очищена."})
		return tryEdit(c, "Изображения удалены.", buildEditMediaMenu(), tele.ModeHTML)
	}
	if data == cbEditMediaAdd {
		setAdminState(userID, STATE_EDIT_MEDIA_ADD)
		return tryEdit(c, "Ожидаю изображения для пополнения галереи.", buildEditMediaMenu(), tele.ModeHTML)
	}
	if data == cbBackToEditRecord {
		setAdminState(userID, STATE_IDLE)
		adminStatesMu.Lock()
		id, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if ok {
			w, err := womanManager.GetWomanByID(id)
			if err != nil || w == nil {
				return tryEdit(c, "Запись не обнаружена.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
			}
			return sendEditMenu(c, w)
		}
		return HandleAdminPanel(c)
	}

	if strings.HasPrefix(data, "do_edit_") {
		action := strings.TrimPrefix(data, "do_edit_")
		adminStatesMu.Lock()
		targetID, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			return tryEdit(c, "Время сессии истекло.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		if action == "delete" {
			if !hasPermission(userID, PermDelete) {
				return tryEdit(c, "Недостаточно прав.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
			}
			setPendingAction(userID, pendingAction{Action: "delete", TargetID: targetID})
			setAdminState(userID, STATE_WAITING_CONFIRM)
			return tryEdit(c, "Подтвердите удаление записи из архива.", buildConfirmMenu(), tele.ModeHTML)
		} else {
			adminStatesMu.Lock()
			adminEditField[userID] = action
			adminStatesMu.Unlock()
			setAdminState(userID, STATE_EDIT_VALUE)
			return tryEdit(c, "Введите новые данные:", buildCancelEditMenu(), tele.ModeHTML)
		}
	}

	if data == cbCancelSuggest {
		setAdminState(userID, STATE_IDLE)
		return tryEdit(c, "Действие отменено.", buildMainMenu(userID), tele.ModeHTML)
	}

	// --- USER CATEGORY SELECT ---
	if strings.HasPrefix(data, "field_more_") {
		field := strings.TrimPrefix(data, "field_more_")
		c.Respond()
		return sendFieldSelection(c, field, true)
	}
	if data == "field_back" {
		c.Respond()
		return HandleUserWoman(c)
	}
	if strings.HasPrefix(data, "field_") {
		c.Respond()
		field := strings.TrimPrefix(data, "field_")
		return sendFieldSelection(c, field, false)
	}
	if data == cbThemeMore {
		if c.Sender() == nil {
			return c.Respond()
		}
		theme, ok := getLastTheme(c.Sender().ID)
		if !ok || theme == "" {
			return c.Respond(&tele.CallbackResponse{Text: "Сначала вызовите /theme"})
		}
		items := womanManager.GetRandomWomenByField(theme, 3)
		if len(items) == 0 {
			return c.Respond(&tele.CallbackResponse{Text: "Пусто"})
		}
		for i, w := range items {
			_ = sendCardToUser(c, &w, i == len(items)-1)
			time.Sleep(120 * time.Millisecond)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "search_tag_") {
		idxStr := strings.TrimPrefix(data, "search_tag_")
		idx, _ := strconv.Atoi(idxStr)
		if s, ok := getSearchSuggestion(userID); ok && idx >= 0 && idx < len(s.Tags) {
			f := SearchFilters{Tags: []string{s.Tags[idx]}, Limit: 10, PublishedOnly: true}
			results := womanManager.SearchWomenAdvanced(f)
			return sendSearchResults(c, results)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "search_field_") {
		idxStr := strings.TrimPrefix(data, "search_field_")
		idx, _ := strconv.Atoi(idxStr)
		if s, ok := getSearchSuggestion(userID); ok && idx >= 0 && idx < len(s.Fields) {
			f := SearchFilters{Field: s.Fields[idx], Limit: 10, PublishedOnly: true}
			results := womanManager.SearchWomenAdvanced(f)
			return sendSearchResults(c, results)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "col_show_") {
		idStr := strings.TrimPrefix(data, "col_show_")
		id, _ := strconv.Atoi(idStr)
		if id > 0 {
			return sendCollection(c, uint(id), false)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "col_more_") {
		idStr := strings.TrimPrefix(data, "col_more_")
		id, _ := strconv.Atoi(idStr)
		if id > 0 {
			return sendCollection(c, uint(id), true)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "browse_century_") {
		centStr := strings.TrimPrefix(data, "browse_century_")
		cent, _ := strconv.Atoi(centStr)
		if cent > 0 {
			setBrowseState(userID, browseState{YearFrom: (cent-1)*100 + 1, YearTo: cent * 100})
			return sendBrowseFields(c, 0, true)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "browse_centuries_page_") {
		pstr := strings.TrimPrefix(data, "browse_centuries_page_")
		p, _ := strconv.Atoi(pstr)
		return sendBrowseCentury(c, p, true)
	}
	if strings.HasPrefix(data, "browse_fields_page_") {
		pstr := strings.TrimPrefix(data, "browse_fields_page_")
		p, _ := strconv.Atoi(pstr)
		return sendBrowseFields(c, p, true)
	}
	if strings.HasPrefix(data, "browse_field_") {
		idxStr := strings.TrimPrefix(data, "browse_field_")
		idx, _ := strconv.Atoi(idxStr)
		cache, ok := getBrowseCache(userID)
		if ok && idx >= 0 && idx < len(cache.Fields) {
			st, _ := getBrowseState(userID)
			st.Field = cache.Fields[idx]
			setBrowseState(userID, st)
			return sendBrowseTags(c, 0, true)
		}
		return c.Respond()
	}
	if strings.HasPrefix(data, "browse_tags_page_") {
		pstr := strings.TrimPrefix(data, "browse_tags_page_")
		p, _ := strconv.Atoi(pstr)
		return sendBrowseTags(c, p, true)
	}
	if strings.HasPrefix(data, "browse_tag_") {
		idxStr := strings.TrimPrefix(data, "browse_tag_")
		idx, _ := strconv.Atoi(idxStr)
		cache, ok := getBrowseCache(userID)
		if ok && idx >= 0 && idx < len(cache.Tags) {
			st, _ := getBrowseState(userID)
			st.Tag = cache.Tags[idx]
			setBrowseState(userID, st)
			return sendBrowseResults(c, false)
		}
		return c.Respond()
	}
	if data == "browse_more" {
		return sendBrowseResults(c, true)
	}
	if data == "browse_back_centuries" {
		return sendBrowseCentury(c, 0, true)
	}
	if data == "browse_back_fields" {
		return sendBrowseFields(c, 0, true)
	}
	return nil
}

// ==========================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ И ХЕНДЛЕРЫ
// ==========================================

func HandleSendInfo(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermBroadcast) {
		return nil
	}
	if ok, wait := checkAdminCooldown(c.Sender().ID, "broadcast", 10*time.Minute); !ok {
		return c.Reply(fmt.Sprintf("Подождите %s перед новой рассылкой.", formatDuration(wait)), tele.ModeHTML)
	}
	args := c.Args()
	if len(args) == 0 {
		return c.Reply("⚠️ Ошибка синтаксиса.\nИспользуйте: <code>/sendinfo Текст</code>", tele.ModeHTML)
	}
	messageText := strings.Join(args, " ")
	startBroadcast(c.Bot(), c.Sender().ID, messageText)
	return nil
}

func makeFieldsMenu() *tele.ReplyMarkup {
	fieldsMenu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, cat := range defaultCategories {
		btnText := cat
		if len([]rune(cat)) > 30 {
			btnText = string([]rune(cat)[:28]) + ".."
		}
		btn := fieldsMenu.Data(btnText, fmt.Sprintf("set_cat_%d", i))
		rows = append(rows, fieldsMenu.Row(btn))
	}
	rows = append(rows, fieldsMenu.Row(fieldsMenu.Data("Прервать", cbCancelSuggest)))
	fieldsMenu.Inline(rows...)
	return fieldsMenu
}

func sendCaptcha(c tele.Context) error {
	a := rand.Intn(5) + 1
	b := rand.Intn(5) + 1
	res := a + b
	options := []int{res, res + 1, res - 1}
	rand.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
	menu := &tele.ReplyMarkup{}
	var btns []tele.Btn
	for _, opt := range options {
		data := "captcha_wrong"
		if opt == res {
			data = "captcha_correct"
		}
		btns = append(btns, menu.Data(strconv.Itoa(opt), data))
	}
	menu.Inline(menu.Row(btns...))
	return c.Send(fmt.Sprintf("🛡 <b>Проверка на человечность.</b>\nРешите пример: %d + %d = ?", a, b), menu, tele.ModeHTML)
}

func Middleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			chat := c.Chat()

			if chat != nil {
				womanManager.SaveKnownChat(chat)
			}
			if sender == nil {
				return next(c)
			}
			isStaffUser := isStaff(sender.ID)

			// Rate Limit
			userLastReqMu.Lock()
			last, exists := userLastReq[sender.ID]
			if !isStaffUser && exists && time.Since(last) < 1*time.Second {
				userLastReqMu.Unlock()
				if c.Message() != nil {
					c.Delete()
				}
				return nil
			}
			userLastReq[sender.ID] = time.Now()
			userLastReqMu.Unlock()
			if isStaffUser {
				return next(c)
			}

			// Captcha
			if !womanManager.IsUserVerified(sender.ID) {
				if c.Callback() != nil && strings.HasPrefix(c.Callback().Data, "captcha_") {
					return next(c)
				}
				if c.Message() != nil && c.Message().Text == "/start" {
					return sendCaptcha(c)
				}
				if c.Message() != nil {
					c.Delete()
				}
				return nil
			}
			return next(c)
		}
	}
}

func startUserStateCollector() {
	userStateGCOnce.Do(func() {
		safeGo("user-state-gc", func() {
			cleanupInactiveUserState(userStateMaxIdle)

			ticker := time.NewTicker(userStateGCInterval)
			defer ticker.Stop()

			for range ticker.C {
				cleanupInactiveUserState(userStateMaxIdle)
			}
		})
	})
}

func cleanupInactiveUserState(maxIdle time.Duration) {
	cutoff := time.Now().Add(-maxIdle)
	activeUsers := make(map[int64]struct{})

	userLastReqMu.Lock()
	for userID, lastSeen := range userLastReq {
		if lastSeen.Before(cutoff) {
			delete(userLastReq, userID)
			continue
		}
		activeUsers[userID] = struct{}{}
	}
	userLastReqMu.Unlock()

	isInactive := func(userID int64) bool {
		_, ok := activeUsers[userID]
		return !ok
	}

	adminStatesMu.Lock()
	for userID := range adminStates {
		if isInactive(userID) {
			delete(adminStates, userID)
		}
	}
	for userID := range adminEditTarget {
		if isInactive(userID) {
			delete(adminEditTarget, userID)
		}
	}
	for userID := range adminEditField {
		if isInactive(userID) {
			delete(adminEditField, userID)
		}
	}
	adminStatesMu.Unlock()

	userLastShownMu.Lock()
	for userID := range userLastShown {
		if isInactive(userID) {
			delete(userLastShown, userID)
		}
	}
	userLastShownMu.Unlock()

	quizStatesMu.Lock()
	for userID := range quizStates {
		if isInactive(userID) {
			delete(quizStates, userID)
		}
	}
	quizStatesMu.Unlock()

	userLastThemeMu.Lock()
	for userID := range userLastTheme {
		if isInactive(userID) {
			delete(userLastTheme, userID)
		}
	}
	userLastThemeMu.Unlock()

	pendingActionsMu.Lock()
	for userID := range pendingActions {
		if isInactive(userID) {
			delete(pendingActions, userID)
		}
	}
	pendingActionsMu.Unlock()

	adminActionLastMu.Lock()
	for userID := range adminActionLast {
		if isInactive(userID) {
			delete(adminActionLast, userID)
		}
	}
	adminActionLastMu.Unlock()

	searchSuggestMu.Lock()
	for userID := range searchSuggest {
		if isInactive(userID) {
			delete(searchSuggest, userID)
		}
	}
	searchSuggestMu.Unlock()

	browseStateMu.Lock()
	for userID := range browseStates {
		if isInactive(userID) {
			delete(browseStates, userID)
		}
	}
	browseStateMu.Unlock()

	browseCacheMu.Lock()
	for userID := range browseCaches {
		if isInactive(userID) {
			delete(browseCaches, userID)
		}
	}
	browseCacheMu.Unlock()
}

func setPendingAction(userID int64, action pendingAction) {
	pendingActionsMu.Lock()
	pendingActions[userID] = action
	pendingActionsMu.Unlock()
}

func getPendingAction(userID int64) (pendingAction, bool) {
	pendingActionsMu.Lock()
	defer pendingActionsMu.Unlock()
	act, ok := pendingActions[userID]
	return act, ok
}

func clearPendingAction(userID int64) {
	pendingActionsMu.Lock()
	delete(pendingActions, userID)
	pendingActionsMu.Unlock()
}

func checkAdminCooldown(userID int64, action string, min time.Duration) (bool, time.Duration) {
	adminActionLastMu.Lock()
	defer adminActionLastMu.Unlock()
	m, ok := adminActionLast[userID]
	if !ok {
		m = make(map[string]time.Time)
		adminActionLast[userID] = m
	}
	last, exists := m[action]
	if exists {
		elapsed := time.Since(last)
		if elapsed < min {
			return false, min - elapsed
		}
	}
	m[action] = time.Now()
	return true, 0
}

func extractID(text string) int64 {
	re := regexp.MustCompile(`\d+`)
	m := re.FindString(text)
	if m == "" {
		return 0
	}
	id, _ := strconv.ParseInt(m, 10, 64)
	return id
}

func normalizeBotCommand(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	cmd := strings.TrimPrefix(fields[0], "/")
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	return strings.ToLower(strings.TrimSpace(cmd))
}

func normalizeNameForDup(name string) string {
	name = strings.ToLower(name)
	name = splitRegex.ReplaceAllString(name, " ")
	name = strings.Join(strings.Fields(name), " ")
	return strings.TrimSpace(name)
}

func setSearchSuggestion(userID int64, s searchSuggestion) {
	searchSuggestMu.Lock()
	searchSuggest[userID] = s
	searchSuggestMu.Unlock()
}

func getSearchSuggestion(userID int64) (searchSuggestion, bool) {
	searchSuggestMu.Lock()
	defer searchSuggestMu.Unlock()
	s, ok := searchSuggest[userID]
	return s, ok
}

func setLastTheme(userID int64, theme string) {
	userLastThemeMu.Lock()
	userLastTheme[userID] = theme
	userLastThemeMu.Unlock()
}

func getLastTheme(userID int64) (string, bool) {
	userLastThemeMu.Lock()
	defer userLastThemeMu.Unlock()
	t, ok := userLastTheme[userID]
	return t, ok
}

func setBrowseState(userID int64, st browseState) {
	browseStateMu.Lock()
	browseStates[userID] = st
	browseStateMu.Unlock()
}

func getBrowseState(userID int64) (browseState, bool) {
	browseStateMu.Lock()
	defer browseStateMu.Unlock()
	st, ok := browseStates[userID]
	return st, ok
}

func setBrowseCache(userID int64, cache browseCache) {
	browseCacheMu.Lock()
	browseCaches[userID] = cache
	browseCacheMu.Unlock()
}

func getBrowseCache(userID int64) (browseCache, bool) {
	browseCacheMu.Lock()
	defer browseCacheMu.Unlock()
	c, ok := browseCaches[userID]
	return c, ok
}

func executePendingAction(c tele.Context) error {
	user := c.Sender()
	if user == nil {
		return nil
	}
	act, ok := getPendingAction(user.ID)
	if !ok {
		return c.Send("Нет ожидающего подтверждения.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
	clearPendingAction(user.ID)
	setAdminState(user.ID, STATE_IDLE)
	switch act.Action {
	case "delete":
		if err := womanManager.DeleteWoman(act.TargetID); err != nil {
			log.Printf("⚠️ Ошибка удаления записи: %v", err)
			return c.Send("Ошибка удаления записи.")
		}
		logModAction(user.ID, "delete", fmt.Sprintf("%d", act.TargetID), "")
		return c.Send("Запись изъята из архива.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	case "merge":
		if err := mergeWomen(act.KeepID, act.RemoveID, user.ID); err != nil {
			return c.Send("Ошибка слияния: "+err.Error(), tele.ModeHTML)
		}
		logModAction(user.ID, "merge", fmt.Sprintf("%d/%d", act.KeepID, act.RemoveID), "")
		return c.Send("Слияние завершено.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	case "tagadd", "tagremove":
		updated, err := bulkTagUpdate(act.Tag, act.Filters, act.AddTags, user.ID)
		if err != nil {
			return c.Send("Ошибка массового обновления: "+err.Error(), tele.ModeHTML)
		}
		action := "tag_add"
		if !act.AddTags {
			action = "tag_remove"
		}
		logModAction(user.ID, action, act.Tag, fmt.Sprintf("updated %d", updated))
		return c.Send(fmt.Sprintf("Готово. Обновлено записей: %d", updated), buildStaffPanelMenuForContext(c), tele.ModeHTML)
	case cbDBImport:
		if act.FilePath == "" {
			return c.Send("Не найден файл для импорта.")
		}
		if err := replaceDatabase(act.FilePath); err != nil {
			return c.Send("Ошибка замены базы данных.")
		}
		logModAction(user.ID, cbDBImport, "", "confirmed")
		return c.Send("Хранилище знаний успешно обновлено.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	default:
		return c.Send("Неизвестное действие.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
}

func replaceDatabase(tempName string) error {
	if err := womanManager.CloseDB(); err != nil {
		log.Printf("⚠️ Ошибка закрытия БД: %v", err)
	}
	if err := os.MkdirAll(dirBackups, 0755); err != nil {
		log.Printf("⚠️ Ошибка создания каталога бэкапов: %v", err)
	}
	if err := os.Rename(dbFilePath, dbBackupFilePath); err != nil {
		log.Printf("⚠️ Ошибка бэкапа БД: %v", err)
	}
	if err := os.Rename(tempName, dbFilePath); err != nil {
		log.Printf("⚠️ Ошибка замены БД: %v", err)
		return err
	}
	womanManager.Connect()
	return nil
}

func HandleStart(c tele.Context) error {
	if c.Chat() == nil || c.Sender() == nil {
		return nil
	}
	if c.Chat().Type == tele.ChatPrivate {
		if isStaff(c.Sender().ID) {
			setAdminState(c.Sender().ID, STATE_IDLE)
		}
		welcomeText := "Приветствую, путник. Я — Офелия.\n\nЗдесь хранятся истории о великих женщинах. Изучайте архив, проходите испытания знаний и пополняйте летопись."
		if isAdmin(c.Sender().ID) {
			welcomeText += "\n\nДля расширенных административных действий используйте /admin."
		}
		return c.Send(welcomeText, buildMainMenu(c.Sender().ID), tele.ModeHTML)
	}
	return c.Send("Приветствую, путник. Используйте /start в личном чате для меню.", tele.ModeHTML)
}
func HandleHelp(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}

	userHelp := "Команды пользователя:\n" +
		"/start — открыть главное меню\n" +
		"/help — справка\n" +
		"/random — случайная запись\n" +
		"/selection — подборка дня\n" +
		"/theme — тема недели\n" +
		"/era, /century — навигация по времени\n" +
		"/tags, /browse — навигация по тегам\n" +
		"/collections — опубликованные коллекции\n" +
		"/fav, /rec — избранное и рекомендации\n" +
		"/daily_on, /daily_off, /daily_time — ежедневник\n\n" +
		"Меню:\n" +
		"Главное: Сайт, Развлечения\n" +
		"Сайт: Главная, О себе, Проекты, Навыки, Контакты\n" +
		"Развлечения: Женщина, Статистика"

	if !isAdmin(c.Sender().ID) {
		return c.Send(userHelp, tele.ModeHTML)
	}

	adminHelp := userHelp + "\n\nАдмин-команды:\n" +
		"/admin — панель управления\n" +
		"/status, /audit, /history, /broadcasts — диагностика и отчеты\n" +
		"/whitelist, /whitelist_del — белый список\n" +
		"/cms_site — выдать JWT-ссылку на сайт\n" +
		"/cms_post — создать пост\n" +
		"/cms_post_del — удалить пост\n" +
		"/cms_event_add — добавить мероприятие\n" +
		"/event_manage — управление записями мероприятия"

	return c.Send(adminHelp, tele.ModeHTML)
}

func HandleCMSPostCommand(c tele.Context) error {
	if cmsService == nil {
		return c.Send("CMS-репозиторий не инициализирован.", tele.ModeHTML)
	}
	return cmsService.HandleBotCreatePost(c)
}

func HandleCMSEventManageCommand(c tele.Context) error {
	if cmsService == nil {
		return c.Send("CMS-репозиторий не инициализирован.", tele.ModeHTML)
	}
	return cmsService.HandleBotEventManage(c)
}

func HandleCMSEventAddCommand(c tele.Context) error {
	if cmsService == nil {
		return c.Send("CMS-репозиторий не инициализирован.", tele.ModeHTML)
	}
	return cmsService.HandleBotEventAdd(c)
}

func HandleCMSPostDelCommand(c tele.Context) error {
	if cmsService == nil {
		return c.Send("CMS-репозиторий не инициализирован.", tele.ModeHTML)
	}
	return cmsService.HandleBotPostDelete(c)
}

func HandleCMSSiteCommand(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return c.Send("Недостаточно прав.", tele.ModeHTML)
	}

	token, err := generateCMSJWT(c.Sender().ID, true)
	if err != nil {
		log.Printf("⚠️ Ошибка генерации CMS JWT: %v", err)
		return c.Send("Не удалось сгенерировать ссылку. Проверьте OPHELIA_CMS_JWT_SECRET.", tele.ModeHTML)
	}

	link, err := buildCMSSiteURLWithToken(config.CMSSiteURL, token)
	if err != nil {
		log.Printf("⚠️ Ошибка сборки CMS URL: %v", err)
		return c.Send("Некорректный CMS URL. Проверьте OPHELIA_CMS_SITE_URL.", tele.ModeHTML)
	}

	return c.Send(
		fmt.Sprintf("🔐 Ссылка для входа в CMS:\n<a href=\"%s\">%s</a>", html.EscapeString(link), html.EscapeString(link)),
		tele.ModeHTML,
	)
}
func HandleAdminPanel(c tele.Context) error {
	if c.Chat() == nil || c.Sender() == nil {
		return nil
	}
	resetCMSAdminState(c.Sender().ID)
	if c.Chat().Type == tele.ChatPrivate && isStaff(c.Sender().ID) {
		return showStaffPanel(c, false)
	}
	return nil
}
func HandleStartSuggest(c tele.Context) error {
	womanManager.StartAdding(c.Sender().ID)
	setAdminState(c.Sender().ID, STATE_WOMAN_NAME)
	return c.Send("Вы решили пополнить архив (Шаг 1).\n\nНазовите Имя и Фамилию:", buildCancelSuggestMenu(), tele.ModeHTML)
}

func resetCMSAdminState(userID int64) {
	if userID <= 0 || cmsService == nil {
		return
	}
	cmsService.SetState(userID, "")
	cmsService.ResetDraft(userID)
}

func HandleBackToMain(c tele.Context) error {
	if c.Sender() != nil {
		resetCMSAdminState(c.Sender().ID)
		setAdminState(c.Sender().ID, STATE_IDLE)
	}
	if c.Callback() != nil {
		if c.Sender() != nil && isStaff(c.Sender().ID) {
			return showStaffPanel(c, true)
		}
		return showMainInlineMenu(c, true)
	}
	if c.Message() != nil && c.Message().Photo != nil {
		c.Delete()
	}
	return HandleAdminPanel(c)
}

func showStaffPanel(c tele.Context, edit bool) error {
	if c.Chat() == nil || c.Sender() == nil || c.Chat().Type != tele.ChatPrivate || !isStaff(c.Sender().ID) {
		return nil
	}
	resetCMSAdminState(c.Sender().ID)
	setAdminState(c.Sender().ID, STATE_IDLE)
	menu := buildStaffPanelMenu(c.Sender().ID)

	if isAdmin(c.Sender().ID) {
		if edit {
			return tryEdit(c, "Приветствую. Панель управления активирована.", menu, tele.ModeHTML)
		}
		return c.Send("Приветствую. Панель управления активирована.", menu, tele.ModeHTML)
	}
	if edit {
		return tryEdit(c, "Приветствую. Модераторская панель активирована.", menu, tele.ModeHTML)
	}
	return c.Send("Приветствую. Модераторская панель активирована.", menu, tele.ModeHTML)
}

func buildUserStatsText(userID int64) string {
	text := statsManager.GetUserStats(userID)
	ach := getUserAchievements(userID)
	if len(ach) > 0 {
		text += "\n\n🏅 <b>Достижения</b>\n"
		for _, a := range ach {
			text += "• " + a + "\n"
		}
	}
	return text
}

func HandleMe(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	return c.Reply(buildUserStatsText(c.Sender().ID), tele.ModeHTML)
}
func HandleTop(c tele.Context) error { return c.Reply(gameManager.GetTopPlayers(), tele.ModeHTML) }
func HandleStatus(c tele.Context) error {
	return sendStatus(c, false)
}
func HandleAudit(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermAudit) {
		return nil
	}
	return sendAudit(c, false)
}
func HandleWhitelist(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermWhitelist) {
		return nil
	}
	return sendWhitelistPage(c, 0, false)
}
func HandleWhitelistAdd(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermWhitelist) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /wladd <code>&lt;chat_id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.ParseInt(args[0], 10, 64)
	if id == 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	if addWhitelist(id) {
		_ = saveWhitelist()
		logModAction(c.Sender().ID, "whitelist_add", fmt.Sprintf("%d", id), "")
	}
	return c.Reply("Готово.", tele.ModeHTML)
}
func HandleWhitelistDel(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermWhitelist) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /whitelist_del <code>&lt;chat_id&gt;</code> (или /wldel <chat_id>)", tele.ModeHTML)
	}
	id := extractID(args[0])
	if id == 0 {
		return c.Reply("Неверный идентификатор для удаления из whitelist.json.", tele.ModeHTML)
	}
	if removeWhitelist(id) {
		_ = saveWhitelist()
		logModAction(c.Sender().ID, "whitelist_remove", fmt.Sprintf("%d", id), "")
	}
	return c.Reply("Готово.", tele.ModeHTML)
}
func HandleChats(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermViewChats) {
		return nil
	}
	return sendChatsPage(c, 0, false)
}

func HandleCollections(c tele.Context) error {
	cols := womanManager.ListCollections(true)
	if len(cols) == 0 {
		return c.Reply("Коллекций пока нет.", tele.ModeHTML)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, col := range cols {
		btn := menu.Data(col.Name, fmt.Sprintf("col_show_%d", col.ID))
		rows = append(rows, menu.Row(btn))
	}
	menu.Inline(rows...)
	return c.Reply("📚 <b>Коллекции</b>", menu, tele.ModeHTML)
}

func sendCollection(c tele.Context, id uint, more bool) error {
	col, err := womanManager.GetCollection(id)
	if err != nil || col == nil {
		return c.Reply("Коллекция не найдена.", tele.ModeHTML)
	}
	if !col.IsPublished && (c.Sender() == nil || !isAdmin(c.Sender().ID)) {
		return c.Reply("Коллекция скрыта.", tele.ModeHTML)
	}
	if !more {
		header := fmt.Sprintf("📚 <b>%s</b>\n%s", html.EscapeString(col.Name), html.EscapeString(col.Description))
		c.Send(header, tele.ModeHTML)
	}
	items := womanManager.GetRandomWomenByFilters(collectionToFilters(col), 5)
	if len(items) == 0 {
		return c.Reply("В коллекции пока пусто.", tele.ModeHTML)
	}
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	menu := &tele.ReplyMarkup{}
	btn := menu.Data("Еще", fmt.Sprintf("col_more_%d", col.ID))
	menu.Inline(menu.Row(btn))
	return c.Send("Еще голоса этой коллекции:", menu, tele.ModeHTML)
}

func HandleCollectionAdd(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermCollections) {
		return nil
	}
	raw := strings.TrimPrefix(c.Message().Text, "/coladd")
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) < 2 {
		return c.Reply("Используйте: /coladd Название | Описание | tag:... field:\"...\" year:1800-1900", tele.ModeHTML)
	}
	name := strings.TrimSpace(parts[0])
	desc := strings.TrimSpace(parts[1])
	filterText := ""
	if len(parts) == 3 {
		filterText = strings.TrimSpace(parts[2])
	}
	filters, errMsg := parseSearchFilters(tokenizeSearchArgs(filterText))
	if errMsg != "" && filterText != "" {
		return c.Reply(errMsg, tele.ModeHTML)
	}
	col := &Collection{
		Name:        name,
		Description: desc,
		Field:       filters.Field,
		Tags:        filters.Tags,
		YearFrom:    filters.YearFrom,
		YearTo:      filters.YearTo,
		IsPublished: true,
	}
	if err := womanManager.CreateCollection(col); err != nil {
		return c.Reply("Ошибка создания коллекции: "+err.Error(), tele.ModeHTML)
	}
	logModAction(c.Sender().ID, "collection_add", fmt.Sprintf("%d", col.ID), name)
	return c.Reply("Коллекция создана.", tele.ModeHTML)
}

func HandleCollectionDel(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermCollections) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /coldel <code>&lt;id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.Atoi(args[0])
	if id <= 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	if err := womanManager.DeleteCollection(uint(id)); err != nil {
		return c.Reply("Ошибка удаления: "+err.Error(), tele.ModeHTML)
	}
	logModAction(c.Sender().ID, "collection_del", fmt.Sprintf("%d", id), "")
	return c.Reply("Коллекция удалена.", tele.ModeHTML)
}

func HandleCollectionList(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermCollections) {
		return nil
	}
	cols := womanManager.ListCollections(false)
	if len(cols) == 0 {
		return c.Reply("Коллекций нет.", tele.ModeHTML)
	}
	var sb strings.Builder
	sb.WriteString("📚 <b>Коллекции</b>\n")
	for _, col := range cols {
		status := "published"
		if !col.IsPublished {
			status = "hidden"
		}
		sb.WriteString(fmt.Sprintf("• %d — %s (%s)\n", col.ID, html.EscapeString(col.Name), status))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}

func HandleCollectionPublish(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermCollections) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /colpub <code>&lt;id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.Atoi(args[0])
	if id <= 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	col, err := womanManager.GetCollection(uint(id))
	if err != nil || col == nil {
		return c.Reply("Коллекция не найдена.", tele.ModeHTML)
	}
	col.IsPublished = true
	_ = womanManager.UpdateCollection(col)
	return c.Reply("Коллекция опубликована.", tele.ModeHTML)
}

func HandleCollectionUnpublish(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermCollections) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /colunpub <code>&lt;id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.Atoi(args[0])
	if id <= 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	col, err := womanManager.GetCollection(uint(id))
	if err != nil || col == nil {
		return c.Reply("Коллекция не найдена.", tele.ModeHTML)
	}
	col.IsPublished = false
	_ = womanManager.UpdateCollection(col)
	return c.Reply("Коллекция скрыта.", tele.ModeHTML)
}

func sendWhitelistPage(c tele.Context, page int, edit bool) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermWhitelist) {
		return nil
	}
	ids := listWhitelist()
	if len(ids) == 0 {
		msg := "Белый список пуст."
		if edit {
			return tryEdit(c, msg, buildCancelEditMenu(), tele.ModeHTML)
		}
		return c.Send(msg, tele.ModeHTML)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	pageSize := 8
	totalPages := (len(ids) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(ids) {
		end = len(ids)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ <b>Белый список</b> (страница %d/%d)\n\n", page+1, totalPages))
	for _, id := range ids[start:end] {
		title := formatChatName(id)
		if title != "" {
			sb.WriteString(fmt.Sprintf("• %d — %s\n", id, html.EscapeString(title)))
		} else {
			sb.WriteString(fmt.Sprintf("• %d\n", id))
		}
	}

	wlMenu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, id := range ids[start:end] {
		btn := wlMenu.Data(fmt.Sprintf("Удалить %d", id), fmt.Sprintf("wl_del_%d", id))
		rows = append(rows, wlMenu.Row(btn))
	}
	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, wlMenu.Data("⬅️ Назад", fmt.Sprintf("wl_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, wlMenu.Data("Вперед ➡️", fmt.Sprintf("wl_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, wlMenu.Row(nav...))
	}
	rows = append(rows, wlMenu.Row(wlMenu.Data("Добавить", "wl_add")))
	wlMenu.Inline(rows...)
	if edit {
		return tryEdit(c, sb.String(), wlMenu, tele.ModeHTML)
	}
	return c.Send(sb.String(), wlMenu, tele.ModeHTML)
}

func sendChatsPage(c tele.Context, page int, edit bool) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermViewChats) {
		return nil
	}
	pageSize := 8
	offset := page * pageSize
	chats, total := womanManager.ListKnownChats(pageSize, offset)
	if total == 0 {
		msg := "Список чатов пуст."
		if edit {
			return tryEdit(c, msg, buildCancelEditMenu(), tele.ModeHTML)
		}
		return c.Send(msg, tele.ModeHTML)
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📒 <b>Чаты с ботом</b> (страница %d/%d)\n\n", page+1, totalPages))
	for _, ch := range chats {
		name := ch.Title
		if name == "" {
			name = ch.Username
		}
		if name == "" {
			name = "-"
		}
		mark := ""
		if isWhitelisted(ch.ID) {
			mark = " ✅"
		}
		u := ""
		if ch.Username != "" {
			u = " @" + ch.Username
		}
		sb.WriteString(fmt.Sprintf("• %d — %s%s [%s]%s\n", ch.ID, html.EscapeString(name), u, ch.Type, mark))
	}
	chMenu := &tele.ReplyMarkup{}
	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, chMenu.Data("⬅️ Назад", fmt.Sprintf("chats_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, chMenu.Data("Вперед ➡️", fmt.Sprintf("chats_page_%d", page+1)))
	}
	if len(nav) > 0 {
		chMenu.Inline(chMenu.Row(nav...))
	}
	if edit {
		return tryEdit(c, sb.String(), chMenu, tele.ModeHTML)
	}
	return c.Send(sb.String(), chMenu, tele.ModeHTML)
}

func formatChatName(id int64) string {
	ch := womanManager.GetKnownChat(id)
	if ch == nil {
		return ""
	}
	title := ch.Title
	if title == "" {
		title = ch.Username
	}
	if ch.Username != "" && title != ch.Username {
		return title + " @" + ch.Username
	}
	return title
}
func HandleBroadcasts(c tele.Context) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	var logs []BroadcastLog
	womanManager.DB.Order("created_at desc").Limit(5).Find(&logs)
	if len(logs) == 0 {
		return c.Reply("Рассылок пока нет.", tele.ModeHTML)
	}
	var sb strings.Builder
	sb.WriteString("📢 <b>Последние рассылки</b>\n\n")
	for _, l := range logs {
		sb.WriteString(fmt.Sprintf("• %s — %d/%d (ошибок: %d)\n",
			l.CreatedAt.Format("02.01 15:04"), l.Success, l.Total, l.Fail))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleInbox(c tele.Context) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	raw := strings.TrimSpace(strings.TrimPrefix(c.Message().Text, "/inbox"))
	args := tokenizeSearchArgs(raw)
	filters := SearchFilters{Limit: 10, UnpublishedOnly: true}
	if len(args) > 0 {
		f, errMsg := parseSearchFilters(args)
		if errMsg != "" {
			return c.Reply(errMsg, tele.ModeHTML)
		}
		f.UnpublishedOnly = true
		f.PublishedOnly = false
		filters = f
	}
	results := womanManager.SearchWomenAdvanced(filters)
	if len(results) == 0 {
		return c.Reply("Входящих записей не найдено.", tele.ModeHTML)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i := 0; i < len(results) && i < 8; i++ {
		w := results[i]
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("select_edit_%d", w.ID))
		rows = append(rows, menu.Row(btn))
	}
	menu.Inline(rows...)
	return c.Reply("Корреспонденция:", menu, tele.ModeHTML)
}
func HandleExport(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	includeAll := len(args) > 0 && args[0] == "all"
	file, err := exportCSV(includeAll)
	if err != nil {
		return c.Reply("Не удалось сформировать экспорт.")
	}
	defer os.Remove(file)
	doc := &tele.Document{File: tele.FromDisk(file), FileName: "women_export.csv"}
	_, err = c.Bot().Send(c.Sender(), doc)
	if err != nil {
		return c.Reply("Ошибка отправки файла.")
	}
	return nil
}
func HandleMerge(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if ok, wait := checkAdminCooldown(c.Sender().ID, "merge", 2*time.Minute); !ok {
		return c.Reply(fmt.Sprintf("Подождите %s перед новой операцией слияния.", formatDuration(wait)), tele.ModeHTML)
	}
	args := c.Args()
	if len(args) != 2 {
		return c.Reply("Используйте: /merge <code>&lt;keep_id&gt; &lt;remove_id&gt;</code>", tele.ModeHTML)
	}
	keepID, _ := strconv.Atoi(args[0])
	removeID, _ := strconv.Atoi(args[1])
	if keepID <= 0 || removeID <= 0 || keepID == removeID {
		return c.Reply("Неверные идентификаторы.", tele.ModeHTML)
	}
	setPendingAction(c.Sender().ID, pendingAction{Action: "merge", KeepID: uint(keepID), RemoveID: uint(removeID)})
	setAdminState(c.Sender().ID, STATE_WAITING_CONFIRM)
	return c.Reply("Подтвердите слияние записей.", buildConfirmMenu(), tele.ModeHTML)
}
func HandleTagAdd(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermMassTag) {
		return nil
	}
	if ok, wait := checkAdminCooldown(c.Sender().ID, "tagadd", 1*time.Minute); !ok {
		return c.Reply(fmt.Sprintf("Подождите %s перед новой операцией.", formatDuration(wait)), tele.ModeHTML)
	}
	tag, filters, errMsg := parseTagCommand(c.Message().Text)
	if errMsg != "" {
		return c.Reply(errMsg, tele.ModeHTML)
	}
	setPendingAction(c.Sender().ID, pendingAction{Action: "tagadd", Tag: tag, Filters: filters, AddTags: true})
	setAdminState(c.Sender().ID, STATE_WAITING_CONFIRM)
	return c.Reply("Подтвердите массовое добавление тегов.", buildConfirmMenu(), tele.ModeHTML)
}
func HandleTagRemove(c tele.Context) error {
	if c.Sender() == nil || !hasPermission(c.Sender().ID, PermMassTag) {
		return nil
	}
	if ok, wait := checkAdminCooldown(c.Sender().ID, "tagremove", 1*time.Minute); !ok {
		return c.Reply(fmt.Sprintf("Подождите %s перед новой операцией.", formatDuration(wait)), tele.ModeHTML)
	}
	tag, filters, errMsg := parseTagCommand(c.Message().Text)
	if errMsg != "" {
		return c.Reply(errMsg, tele.ModeHTML)
	}
	setPendingAction(c.Sender().ID, pendingAction{Action: "tagremove", Tag: tag, Filters: filters, AddTags: false})
	setAdminState(c.Sender().ID, STATE_WAITING_CONFIRM)
	return c.Reply("Подтвердите массовое удаление тегов.", buildConfirmMenu(), tele.ModeHTML)
}
func HandleMediaCheck(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	limit := 50
	if len(c.Args()) == 1 {
		if v, err := strconv.Atoi(c.Args()[0]); err == nil && v > 0 {
			limit = v
		}
	}
	runHeavy("media-check", func() { runMediaCheck(c.Bot(), c.Sender().ID, limit) })
	return c.Reply("Проверка медиа запущена. Результаты придут сообщением.", tele.ModeHTML)
}
func HandleHistory(c tele.Context) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /history <code>&lt;id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.Atoi(args[0])
	if id <= 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	rows := womanManager.GetChangeHistory(uint(id), 5)
	if len(rows) == 0 {
		return c.Reply("История пуста.", tele.ModeHTML)
	}
	var sb strings.Builder
	sb.WriteString("📜 <b>История изменений</b>\n\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("• %s — %s\n", r.CreatedAt.Format("02.01 15:04"), html.EscapeString(r.Field)))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleTagSuggest(c tele.Context) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /tagsuggest <code>&lt;id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.Atoi(args[0])
	if id <= 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	w, err := womanManager.GetWomanByID(uint(id))
	if err != nil || w == nil {
		return c.Reply("Запись не найдена.", tele.ModeHTML)
	}
	tags := womanManager.SuggestTags(w)
	if len(tags) == 0 {
		return c.Reply("Подсказок не найдено.", tele.ModeHTML)
	}
	return c.Reply("Подсказки: "+strings.Join(tags, ", "), tele.ModeHTML)
}
func HandleModAdd(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) < 1 {
		return c.Reply("Используйте: /modadd <code>&lt;user_id&gt;</code> [moderator|editor]", tele.ModeHTML)
	}
	id, _ := strconv.ParseInt(args[0], 10, 64)
	if id == 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	role := "moderator"
	if len(args) > 1 {
		role = strings.ToLower(strings.TrimSpace(args[1]))
		if _, ok := rolePermissions[role]; !ok {
			return c.Reply("Неизвестная роль. Используйте moderator или editor.", tele.ModeHTML)
		}
	}
	if err := womanManager.AddModerator(id, role); err != nil {
		return c.Reply("Не удалось добавить модератора.", tele.ModeHTML)
	}
	logModAction(c.Sender().ID, "mod_add", fmt.Sprintf("%d", id), role)
	return c.Reply("Модератор добавлен.", tele.ModeHTML)
}
func HandleModDel(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /moddel <code>&lt;user_id&gt;</code>", tele.ModeHTML)
	}
	id, _ := strconv.ParseInt(args[0], 10, 64)
	if id == 0 {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	if err := womanManager.RemoveModerator(id); err != nil {
		return c.Reply("Не удалось удалить модератора.", tele.ModeHTML)
	}
	logModAction(c.Sender().ID, "mod_del", fmt.Sprintf("%d", id), "")
	return c.Reply("Модератор удален.", tele.ModeHTML)
}
func HandleModList(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	mods := womanManager.ListModeratorsWithRoles()
	if len(mods) == 0 {
		return c.Reply("Модераторов нет.", tele.ModeHTML)
	}
	var sb strings.Builder
	sb.WriteString("🧭 <b>Модераторы</b>\n")
	for _, m := range mods {
		role := m.Role
		if role == "" {
			role = "moderator"
		}
		sb.WriteString(fmt.Sprintf("• %d — %s\n", m.UserID, role))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleModLog(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	var logs []ModAction
	if len(c.Args()) == 1 {
		id, _ := strconv.ParseInt(c.Args()[0], 10, 64)
		womanManager.DB.Where("user_id = ?", id).Order("created_at desc").Limit(10).Find(&logs)
	} else {
		womanManager.DB.Order("created_at desc").Limit(10).Find(&logs)
	}
	if len(logs) == 0 {
		return c.Reply("Логи пусты.", tele.ModeHTML)
	}
	var sb strings.Builder
	sb.WriteString("🧾 <b>Логи действий</b>\n")
	for _, l := range logs {
		sb.WriteString(fmt.Sprintf("• %s — %d: %s %s\n", l.CreatedAt.Format("02.01 15:04"), l.UserID, l.Action, l.TargetID))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleDuplicates(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	var women []Woman
	womanManager.DB.Select("id", "name", "year", "field").Find(&women)
	if len(women) == 0 {
		return c.Reply("Записей нет.", tele.ModeHTML)
	}
	groups := map[string][]Woman{}
	for _, w := range women {
		key := normalizeNameForDup(w.Name)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], w)
	}
	var sb strings.Builder
	sb.WriteString("🔁 <b>Возможные дубликаты</b>\n")
	count := 0
	for _, list := range groups {
		if len(list) < 2 {
			continue
		}
		count++
		if count > 10 {
			sb.WriteString("... и другие.\n")
			break
		}
		sb.WriteString("• ")
		for i, w := range list {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%d:%s", w.ID, html.EscapeString(shorten(w.Name, 18))))
		}
		sb.WriteString("\n")
	}
	if count == 0 {
		return c.Reply("Дубликатов не найдено.", tele.ModeHTML)
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleQuality(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	var women []Woman
	womanManager.DB.Order("id desc").Limit(300).Find(&women)
	if len(women) == 0 {
		return c.Reply("Записей нет.", tele.ModeHTML)
	}
	type item struct {
		W     Woman
		Score int
	}
	var list []item
	for _, w := range women {
		list = append(list, item{W: w, Score: qualityScore(&w)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score < list[j].Score })
	limit := 10
	if len(list) < limit {
		limit = len(list)
	}
	var sb strings.Builder
	sb.WriteString("🧪 <b>Качество карточек (низкое)</b>\n")
	for i := 0; i < limit; i++ {
		it := list[i]
		sb.WriteString(fmt.Sprintf("• %d (%d/4) — %s\n", it.W.ID, it.Score, html.EscapeString(shorten(it.W.Name, 24))))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}

func HandleTopCards(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	topViews := womanManager.TopWomenByViews(5)
	topFavs := womanManager.TopWomenByFavorites(5)
	var sb strings.Builder
	sb.WriteString("🏆 <b>Топ карточек</b>\n\n👁 Просмотры:\n")
	for _, t := range topViews {
		sb.WriteString(fmt.Sprintf("• %s (%d)\n", html.EscapeString(shorten(t.Name, 24)), t.Count))
	}
	sb.WriteString("\n⭐ Избранное:\n")
	for _, t := range topFavs {
		sb.WriteString(fmt.Sprintf("• %s (%d)\n", html.EscapeString(shorten(t.Name, 24)), t.Count))
	}
	return c.Reply(sb.String(), tele.ModeHTML)
}
func HandleThemeOn(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.ThemeActive = true
	womanManager.UpdateSettings(s)
	return c.Reply("Тематический пост включен.", tele.ModeHTML)
}
func HandleThemeOff(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.ThemeActive = false
	womanManager.UpdateSettings(s)
	return c.Reply("Тематический пост выключен.", tele.ModeHTML)
}
func HandleThemeTime(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if len(c.Args()) != 1 {
		return c.Reply("Используйте: /theme_time 10:00", tele.ModeHTML)
	}
	if _, err := time.Parse("15:04", c.Args()[0]); err != nil {
		return c.Reply("Неверный формат времени.", tele.ModeHTML)
	}
	s, _ := womanManager.GetSettings()
	s.ThemeTime = c.Args()[0]
	womanManager.UpdateSettings(s)
	return c.Reply("Время тематического поста обновлено.", tele.ModeHTML)
}
func HandleThemeDay(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if len(c.Args()) != 1 {
		return c.Reply("Используйте: /theme_day 1 (Пн) ... 7 (Вс)", tele.ModeHTML)
	}
	day, _ := strconv.Atoi(c.Args()[0])
	if day < 1 || day > 7 {
		return c.Reply("Неверный день недели.", tele.ModeHTML)
	}
	s, _ := womanManager.GetSettings()
	s.ThemeWeekday = day % 7
	womanManager.UpdateSettings(s)
	return c.Reply("День тематического поста обновлен.", tele.ModeHTML)
}
func HandleHealthOn(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.HealthActive = true
	womanManager.UpdateSettings(s)
	return c.Reply("Ежедневный отчет включен.", tele.ModeHTML)
}
func HandleHealthOff(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.HealthActive = false
	womanManager.UpdateSettings(s)
	return c.Reply("Ежедневный отчет выключен.", tele.ModeHTML)
}
func HandleHealthTime(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if len(c.Args()) != 1 {
		return c.Reply("Используйте: /health_time 09:30", tele.ModeHTML)
	}
	if _, err := time.Parse("15:04", c.Args()[0]); err != nil {
		return c.Reply("Неверный формат времени.", tele.ModeHTML)
	}
	s, _ := womanManager.GetSettings()
	s.HealthTime = c.Args()[0]
	womanManager.UpdateSettings(s)
	return c.Reply("Время отчета обновлено.", tele.ModeHTML)
}
func HandleReportOn(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.ReportActive = true
	womanManager.UpdateSettings(s)
	return c.Reply("Еженедельный отчет включен.", tele.ModeHTML)
}
func HandleReportOff(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	s, _ := womanManager.GetSettings()
	s.ReportActive = false
	womanManager.UpdateSettings(s)
	return c.Reply("Еженедельный отчет выключен.", tele.ModeHTML)
}
func HandleReportTime(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if len(c.Args()) != 1 {
		return c.Reply("Используйте: /report_time 09:15", tele.ModeHTML)
	}
	if _, err := time.Parse("15:04", c.Args()[0]); err != nil {
		return c.Reply("Неверный формат времени.", tele.ModeHTML)
	}
	s, _ := womanManager.GetSettings()
	s.ReportTime = c.Args()[0]
	womanManager.UpdateSettings(s)
	return c.Reply("Время отчета обновлено.", tele.ModeHTML)
}
func HandleReportDay(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	if len(c.Args()) != 1 {
		return c.Reply("Используйте: /report_day <code>&lt;0-6&gt;</code> (вс=0)", tele.ModeHTML)
	}
	day, _ := strconv.Atoi(c.Args()[0])
	if day < 0 || day > 6 {
		return c.Reply("Неверный день. 0=вс, 1=пн ...", tele.ModeHTML)
	}
	s, _ := womanManager.GetSettings()
	s.ReportWeekday = day
	womanManager.UpdateSettings(s)
	return c.Reply("День отчета обновлен.", tele.ModeHTML)
}
func HandleReload(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	loadModerationLists()
	return c.Reply("Списки загружены. Оплот обновлен.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
}
func HandleVerify(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /verify <code>&lt;user_id&gt;</code>", tele.ModeHTML)
	}
	uid, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	womanManager.SetUserVerified(uid)
	return c.Reply("Доступ подтвержден.", tele.ModeHTML)
}
func HandleUnverify(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /unverify <code>&lt;user_id&gt;</code>", tele.ModeHTML)
	}
	uid, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return c.Reply("Неверный идентификатор.", tele.ModeHTML)
	}
	womanManager.UnsetUserVerified(uid)
	return c.Reply("Доступ отозван.", tele.ModeHTML)
}
func HandleShowStats(c tele.Context) error {
	if statsManager == nil {
		return c.Respond()
	}
	imgData, err := statsManager.GenerateStatsImage()
	if err != nil {
		log.Printf("⚠️ Ошибка генерации статистики: %v", err)
		return c.Respond()
	}
	photo := &tele.Photo{File: tele.FromReader(bytes.NewReader(imgData)), Caption: statsManager.GetFormattedStatsText()}
	c.Delete()
	return c.Send(photo, buildStatsMenu(), tele.ModeHTML)
}
func HandleListWords(c tele.Context) error {
	wordsMu.RLock()
	list := strings.Join(badWords, ", ")
	wordsMu.RUnlock()
	if len(list) > 3000 {
		list = list[:3000] + "..."
	}
	return tryEdit(c, fmt.Sprintf("Индекс запрещенных слов: %s", list), buildWordsMenu(), tele.ModeHTML)
}
func HandleID(c tele.Context) error {
	return c.Reply(fmt.Sprintf("Идентификатор: %d", c.Chat().ID), tele.ModeHTML)
}
func HandleStopGame(c tele.Context) error {
	if !isAdmin(c.Sender().ID) {
		return nil
	}
	gameManager.StopGame()
	return c.Reply("Испытание завершено.")
}
func HandleRandomWoman(c tele.Context) error {
	if c.Chat() == nil {
		return nil
	}
	w := womanManager.GetRandomWoman()
	if w == nil {
		return c.Reply("Архив пока пуст.")
	}
	return sendCardToUser(c, w, true)
}
func HandleSelection(c tele.Context) error {
	if c.Chat() == nil {
		return nil
	}
	selection := womanManager.GetRandomWomen(3)
	if len(selection) == 0 {
		return c.Reply("Архив пока пуст.")
	}
	c.Send("🕯 <b>Подборка дня от Офелии</b>\nТри истории, три зеркала времени.", tele.ModeHTML)
	for i, w := range selection {
		_ = sendCardToUser(c, &w, i == len(selection)-1)
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}
func HandleTheme(c tele.Context) error {
	if c.Chat() == nil {
		return nil
	}
	theme := pickWeeklyTheme()
	if theme == "" {
		return c.Reply("Летопись пока без тем.")
	}
	if c.Sender() != nil {
		setLastTheme(c.Sender().ID, theme)
	}
	c.Send(fmt.Sprintf("🗝 <b>Тема недели:</b> %s", theme), buildThemeMoreMenu(), tele.ModeHTML)
	items := womanManager.GetRandomWomenByField(theme, 3)
	if len(items) == 0 {
		return c.Reply("В этой теме пока пусто.")
	}
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	return nil
}
func HandleTagsMenu(c tele.Context) error {
	return sendTagsPage(c, 0, false)
}
func HandleBrowse(c tele.Context) error {
	return sendBrowseCentury(c, 0, false)
}
func HandleFavorites(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	return sendFavoritesPage(c, c.Sender().ID, 0, false)
}
func HandleRecommendations(c tele.Context) error {
	if c.Sender() == nil || c.Chat() == nil {
		return nil
	}
	recs := buildRecommendations(c.Sender().ID)
	if len(recs) == 0 {
		return c.Reply("Пока нет рекомендаций. Начните с /random.", tele.ModeHTML)
	}
	c.Send("🌊 <b>Рекомендации Офелии</b>", tele.ModeHTML)
	for i, w := range recs {
		_ = sendCardToUser(c, &w, i == len(recs)-1)
		time.Sleep(120 * time.Millisecond)
	}
	return nil
}
func HandleDailyStatus(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	sub, err := womanManager.GetSubscription(c.Sender().ID)
	if err != nil || sub == nil {
		return c.Reply("Ежедневник выключен. Включить: /daily_on", tele.ModeHTML)
	}
	status := "выключен"
	if sub.IsActive {
		status = "включен"
	}
	return c.Reply(fmt.Sprintf("Ежедневник %s. Время: %s", status, sub.Time), tele.ModeHTML)
}
func HandleDailyOn(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	if err := womanManager.SetSubscription(c.Sender().ID, true, "09:00"); err != nil {
		return c.Reply("Не удалось включить ежедневник.")
	}
	return c.Reply("Ежедневник включен. Время: 09:00. Изменить: /daily_time 08:30", tele.ModeHTML)
}
func HandleDailyOff(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	if err := womanManager.SetSubscription(c.Sender().ID, false, "09:00"); err != nil {
		return c.Reply("Не удалось отключить ежедневник.")
	}
	return c.Reply("Ежедневник выключен.", tele.ModeHTML)
}
func HandleDailyTime(c tele.Context) error {
	if c.Sender() == nil {
		return nil
	}
	args := c.Args()
	if len(args) != 1 {
		return c.Reply("Используйте: /daily_time 09:30", tele.ModeHTML)
	}
	if _, err := time.Parse("15:04", args[0]); err != nil {
		return c.Reply("Неверный формат времени. Пример: 09:30", tele.ModeHTML)
	}
	if err := womanManager.SetSubscription(c.Sender().ID, true, args[0]); err != nil {
		return c.Reply("Не удалось обновить время.")
	}
	return c.Reply("Время ежедневника обновлено.", tele.ModeHTML)
}
func HandleEraMenu(c tele.Context) error {
	return sendErasMenu(c, false)
}
func HandleCenturyMenu(c tele.Context) error {
	return sendCenturiesMenu(c, false)
}
func HandleSearch(c tele.Context) error {
	raw := ""
	if c.Message() != nil {
		raw = c.Message().Text
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/search@") {
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) == 2 {
			raw = strings.TrimSpace(parts[1])
		} else {
			raw = ""
		}
	} else if strings.HasPrefix(raw, "/search") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "/search"))
	}
	args := tokenizeSearchArgs(raw)
	if len(args) == 0 {
		// Подсказки
		tags := womanManager.GetTagStats()
		fields := womanManager.GetUniqueFields()
		var tagList []string
		for i := 0; i < len(tags) && i < 6; i++ {
			tagList = append(tagList, tags[i].Tag)
		}
		var fieldList []string
		for i := 0; i < len(fields) && i < 6; i++ {
			fieldList = append(fieldList, fields[i])
		}
		setSearchSuggestion(c.Sender().ID, searchSuggestion{Tags: tagList, Fields: fieldList})
		menu := &tele.ReplyMarkup{}
		var rows []tele.Row
		if len(tagList) > 0 {
			for i, t := range tagList {
				btn := menu.Data("#"+t, fmt.Sprintf("search_tag_%d", i))
				rows = append(rows, menu.Row(btn))
			}
		}
		if len(fieldList) > 0 {
			for i, f := range fieldList {
				btn := menu.Data(f, fmt.Sprintf("search_field_%d", i))
				rows = append(rows, menu.Row(btn))
			}
		}
		menu.Inline(rows...)
		return c.Reply("Используйте: /search <текст> или фильтры:\nname:мария field:\"точные науки\" year:1800-1900 tag:математика century:19\n\nБыстрые фильтры:", menu, tele.ModeHTML)
	}
	filters, errMsg := parseSearchFilters(args)
	if errMsg != "" {
		return c.Reply(errMsg, tele.ModeHTML)
	}
	results := womanManager.SearchWomenAdvanced(filters)
	return sendSearchResults(c, results)
}

func sendSearchResults(c tele.Context, results []Woman) error {
	if len(results) == 0 {
		return c.Reply("Ничего не найдено.", tele.ModeHTML)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	limit := 8
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		w := results[i]
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("user_show_%d", w.ID))
		rows = append(rows, menu.Row(btn))
	}
	menu.Inline(rows...)
	return c.Reply("Результаты поиска:", menu, tele.ModeHTML)
}
func HandleUserJoin(c tele.Context) error {
	if c.Message() == nil {
		return nil
	}
	c.Delete()
	for _, u := range c.Message().UsersJoined {
		if check, r := checkNickname(&u); check {
			banUserImmediately(c, &u, r)
		}
	}
	return nil
}
func HandlePhoto(c tele.Context) error {
	if cmsService != nil {
		if handled, err := cmsService.HandleBotCMSAdminMedia(c); handled {
			return err
		}
	}
	userID := c.Sender().ID
	state := getAdminState(userID)
	if isAdmin(userID) && state == STATE_WAITING_PHOTO {
		gameManager.SetGamePhoto(c.Message().Photo.FileID)
		setAdminState(userID, STATE_WAITING_ANSWER)
		return c.Send("Изображение принято. Укажите верный ответ:", tele.ModeHTML)
	}
	if isAdmin(userID) && state == STATE_EDIT_MEDIA_ADD {
		adminStatesMu.Lock()
		id, ok := adminEditTarget[userID]
		adminStatesMu.Unlock()
		if !ok {
			setAdminState(userID, STATE_IDLE)
			return c.Send("Ошибка доступа.")
		}
		w, err := womanManager.GetWomanByID(id)
		if err != nil || w == nil {
			return c.Send("Запись не обнаружена.")
		}
		w.MediaIDs = append(w.MediaIDs, c.Message().Photo.FileID)
		if err := womanManager.UpdateWoman(w); err != nil {
			log.Printf("⚠️ Ошибка обновления медиа: %v", err)
			return c.Send("Ошибка обновления записи.")
		}
		return c.Send(fmt.Sprintf("Изображение добавлено. Всего: %d", len(w.MediaIDs)))
	}
	if state == STATE_WOMAN_MEDIA {
		count := 0
		if err := womanManager.WithDraft(userID, func(draft *Woman) error {
			draft.MediaIDs = append(draft.MediaIDs, c.Message().Photo.FileID)
			count = len(draft.MediaIDs)
			return nil
		}); err != nil {
			setAdminState(userID, STATE_IDLE)
			return c.Send("Сессия утрачена.")
		}
		menuToSend := buildFinishPhotoMenu()
		if !isAdmin(userID) {
			menuToSend = buildFinishSuggestMenu()
		}
		log.Printf("Photo added. Total: %d", count)
		if count == 1 {
			return c.Send(fmt.Sprintf("Изображение принято (%d). Завершите процесс или добавьте ещё.", count), menuToSend, tele.ModeHTML)
		}
	}
	return nil
}
func HandleDocument(c tele.Context) error {
	if cmsService != nil {
		if handled, err := cmsService.HandleBotCMSAdminMedia(c); handled {
			return err
		}
	}
	userID := c.Sender().ID
	state := getAdminState(userID)
	if hasPermission(userID, PermImportDB) && state == STATE_WAITING_DB_IMPORT && c.Chat().Type == tele.ChatPrivate {
		doc := c.Message().Document
		if doc == nil || (!strings.HasSuffix(doc.FileName, ".db") && !strings.HasSuffix(doc.FileName, ".sqlite")) {
			return c.Send("Формат файла не поддерживается.")
		}
		c.Send("Инициирую процедуру замены...")
		tempName := dbTempImportPath
		if err := c.Bot().Download(&doc.File, tempName); err != nil {
			log.Printf("⚠️ Ошибка загрузки файла БД: %v", err)
			return c.Send("Не удалось загрузить файл.")
		}
		setPendingAction(userID, pendingAction{Action: cbDBImport, FilePath: tempName})
		setAdminState(userID, STATE_WAITING_CONFIRM)
		return c.Send("Подтвердите замену базы данных. Действие необратимо.", buildConfirmMenu(), tele.ModeHTML)
	}
	if state == STATE_WOMAN_MEDIA && strings.HasPrefix(c.Message().Document.MIME, "image/") {
		count := 0
		if err := womanManager.WithDraft(userID, func(draft *Woman) error {
			draft.MediaIDs = append(draft.MediaIDs, c.Message().Document.FileID)
			count = len(draft.MediaIDs)
			return nil
		}); err != nil {
			setAdminState(userID, STATE_IDLE)
			return c.Send("Сессия утрачена.")
		}
		menuToSend := buildFinishPhotoMenu()
		if !isAdmin(userID) {
			menuToSend = buildFinishSuggestMenu()
		}
		if count == 1 {
			return c.Send(fmt.Sprintf("Файл принят (%d). Ожидаю подтверждения.", count), menuToSend, tele.ModeHTML)
		}
		return nil
	}
	return nil
}
func HandleText(c tele.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}
	user := c.Sender()
	text := c.Text()
	if user == nil {
		if c.Chat() != nil && c.Chat().ID == config.TargetChatID {
			statsManager.TrackMessage(c)
		}
		return nil
	}
	chat := c.Chat()
	if chat == nil {
		return nil
	}

	// Единый command-router в стиле switch update.Message.Command():
	// важно завершать обработку через return, чтобы не было дублей.
	if strings.HasPrefix(strings.TrimSpace(text), "/") {
		switch normalizeBotCommand(text) {
		case "help":
			return HandleHelp(c)
		case "cms_post":
			return HandleCMSPostCommand(c)
		case "event_manage":
			return HandleCMSEventManageCommand(c)
		case "cms_event_add":
			return HandleCMSEventAddCommand(c)
		case "cms_post_del":
			return HandleCMSPostDelCommand(c)
		case "cms_site":
			return HandleCMSSiteCommand(c)
		case "whitelist_del":
			return HandleWhitelistDel(c)
		}
	}

	if cmsService != nil {
		if handled, err := cmsService.HandleBotCMSAdminText(c); handled {
			return err
		}
	}

	if chat.Type == tele.ChatPrivate {
		currentState := getAdminState(user.ID)
		if isAdmin(user.ID) {
			if currentState == STATE_WAITING_CONFIRM {
				low := strings.ToLower(strings.TrimSpace(text))
				if low == "да" || low == "подтвердить" || low == "confirm" {
					return executePendingAction(c)
				}
				if low == "отмена" || low == "cancel" {
					clearPendingAction(user.ID)
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Действие отменено.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
				}
				return c.Send("Подтвердите действие кнопкой или словом: ДА.", buildConfirmMenu(), tele.ModeHTML)
			}
			if currentState == STATE_WAITING_WL_ADD {
				id := extractID(text)
				if id == 0 {
					return c.Send("Не удалось распознать ID. Пример: 123456789", buildCancelEditMenu(), tele.ModeHTML)
				}
				if addWhitelist(id) {
					_ = saveWhitelist()
					logModAction(user.ID, "whitelist_add", fmt.Sprintf("%d", id), "")
				}
				setAdminState(user.ID, STATE_IDLE)
				return sendWhitelistPage(c, 0, false)
			}
			if currentState == STATE_WAITING_REJECT {
				adminStatesMu.Lock()
				id, ok := adminEditTarget[user.ID]
				adminStatesMu.Unlock()
				if !ok {
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Ошибка идентификатора.")
				}
				w, err := womanManager.GetWomanByID(id)
				if err != nil || w == nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Запись не найдена.")
				}
				reason := strings.TrimSpace(text)
				if reason == "-" {
					reason = ""
				}
				if err := womanManager.DeleteWoman(id); err != nil {
					log.Printf("⚠️ Ошибка удаления записи: %v", err)
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Ошибка удаления записи.")
				}
				logModAction(user.ID, "reject", fmt.Sprintf("%d", id), reason)
				if w.SuggestedBy != 0 {
					msg := "Ваше предложение не принято."
					if reason != "" {
						msg += "\nПричина: " + reason
					}
					_, _ = c.Bot().Send(&tele.User{ID: w.SuggestedBy}, msg)
				}
				setAdminState(user.ID, STATE_IDLE)
				return c.Send("Запись отклонена.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
			}
			if currentState == STATE_WAITING_TIME {
				if _, err := time.Parse("15:04", text); err != nil {
					return c.Send("Неверный формат времени. Пример: 09:00", buildCancelEditMenu(), tele.ModeHTML)
				}
				s, _ := womanManager.GetSettings()
				s.ScheduleTime = text
				if err := womanManager.UpdateSettings(s); err != nil {
					log.Printf("⚠️ Ошибка обновления времени: %v", err)
				}
				setAdminState(user.ID, STATE_IDLE)
				return sendSettingsMenu(c)
			}
			if currentState == STATE_EDIT_SEARCH {
				results := womanManager.SearchWomen(text)
				if len(results) == 0 {
					return c.Send("Записей не обнаружено.", tele.ModeHTML)
				}
				resultsMenu := &tele.ReplyMarkup{}
				var rows []tele.Row
				for _, w := range results {
					btn := resultsMenu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("select_edit_%d", w.ID))
					rows = append(rows, resultsMenu.Row(btn))
				}
				rows = append(rows, resultsMenu.Row(resultsMenu.Data("Прервать", cbAdminBackMain)))
				resultsMenu.Inline(rows...)
				return c.Send("Результаты поиска:", resultsMenu, tele.ModeHTML)
			}
			if currentState == STATE_EDIT_VALUE {
				adminStatesMu.Lock()
				id, hasID := adminEditTarget[user.ID]
				field, hasField := adminEditField[user.ID]
				adminStatesMu.Unlock()
				if !hasID || !hasField {
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Ошибка.")
				}
				w, err := womanManager.GetWomanByID(id)
				if err != nil || w == nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Send("Запись не найдена.")
				}
				oldVal := ""
				newVal := ""
				switch field {
				case "name":
					oldVal = w.Name
					w.Name = text
					newVal = w.Name
				case "year":
					oldVal = w.Year
					w.Year = text
					newVal = w.Year
				case "field":
					oldVal = w.Field
					w.Field = text
					newVal = w.Field
				case "info":
					oldVal = w.Info
					w.Info = text
					newVal = w.Info
				case "tags":
					oldVal = strings.Join(w.Tags, ", ")
					w.Tags = parseTagsText(text)
					newVal = strings.Join(w.Tags, ", ")
				}
				if err := womanManager.UpdateWoman(w); err != nil {
					log.Printf("⚠️ Ошибка обновления записи: %v", err)
				}
				womanManager.LogChange(user.ID, w.ID, field, oldVal, newVal)
				setAdminState(user.ID, STATE_IDLE)
				ov := shorten(oldVal, 500)
				nv := shorten(newVal, 500)
				if ov == "" {
					ov = "—"
				}
				if nv == "" {
					nv = "—"
				}
				c.Send(fmt.Sprintf("<b>Обновлено</b>\nСтарая: %s\nНовая: %s", html.EscapeString(ov), html.EscapeString(nv)), tele.ModeHTML)
				return sendEditMenu(c, w)
			}
			if currentState == STATE_WAITING_ADD_WORD {
				wordsMu.Lock()
				badWords = append(badWords, strings.ToLower(text))
				wordsMu.Unlock()
				if err := saveWords(); err != nil {
					log.Printf("⚠️ Ошибка сохранения списка слов: %v", err)
				}
				setAdminState(user.ID, STATE_IDLE)
				return c.Reply("Запрет наложен.", buildStaffPanelMenuForContext(c))
			}
			if currentState == STATE_WAITING_REMOVE_WORD {
				needle := strings.ToLower(text)
				removed := false
				wordsMu.Lock()
				filtered := badWords[:0]
				for _, w := range badWords {
					if strings.ToLower(w) == needle && !removed {
						removed = true
						continue
					}
					filtered = append(filtered, w)
				}
				badWords = filtered
				wordsMu.Unlock()
				if err := saveWords(); err != nil {
					log.Printf("⚠️ Ошибка сохранения списка слов: %v", err)
				}
				setAdminState(user.ID, STATE_IDLE)
				if removed {
					return c.Reply("Слово амнистировано.", buildStaffPanelMenuForContext(c))
				}
				return c.Reply("Слово не найдено.", buildStaffPanelMenuForContext(c))
			}
			if currentState == STATE_WAITING_BROADCAST {
				if !hasPermission(user.ID, PermBroadcast) {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Недостаточно прав.", buildStaffPanelMenuForContext(c))
				}
				if ok, wait := checkAdminCooldown(user.ID, "broadcast", 10*time.Minute); !ok {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply(fmt.Sprintf("Подождите %s перед новой рассылкой.", formatDuration(wait)), buildStaffPanelMenuForContext(c))
				}
				setAdminState(user.ID, STATE_IDLE)
				startBroadcast(c.Bot(), user.ID, text)
				return c.Reply("Воззвание отправлено в летопись рассылок.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
			}
			if currentState == STATE_WAITING_ANSWER {
				gameManager.SetGameAnswer(text)
				setAdminState(user.ID, STATE_WAITING_CONTEXT)
				return c.Send("Ответ принят. Введите контекст (дополнительную информацию):", tele.ModeHTML)
			}
			if currentState == STATE_WAITING_CONTEXT {
				gameManager.SetGameContext(text)
				setAdminState(user.ID, STATE_IDLE)
				if err := gameManager.StartGameFromState(c.Bot(), config.TargetChatID); err != nil {
					log.Printf("⚠️ Ошибка старта игры: %v", err)
					return c.Send("Не удалось начать испытание. Проверьте параметры.")
				}
				return c.Send("Испытание началось.")
			}
		}
		if strings.HasPrefix(currentState, "woman_") {
			menuCancel := buildCancelEditMenu()
			if !isAdmin(user.ID) {
				menuCancel = buildCancelSuggestMenu()
			}
			switch currentState {
			case STATE_WOMAN_NAME:
				var name string
				if err := womanManager.WithDraft(user.ID, func(d *Woman) error {
					d.Name = text
					name = d.Name
					return nil
				}); err != nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Сессия истекла.")
				}
				setAdminState(user.ID, STATE_WOMAN_FIELD)
				return c.Send(fmt.Sprintf("Имя принято: <b>%s</b>\nУкажите сферу деятельности (или впишите свой вариант):", name), makeFieldsMenu(), tele.ModeHTML)
			case STATE_WOMAN_FIELD:
				if err := womanManager.WithDraft(user.ID, func(d *Woman) error {
					d.Field = text
					return nil
				}); err != nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Сессия истекла.")
				}
				setAdminState(user.ID, STATE_WOMAN_YEAR)
				return c.Send(fmt.Sprintf("Сфера (ручной ввод): <b>%s</b>\nВведите годы жизни:", text), menuCancel, tele.ModeHTML)
			case STATE_WOMAN_YEAR:
				if err := womanManager.WithDraft(user.ID, func(d *Woman) error {
					d.Year = text
					return nil
				}); err != nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Сессия истекла.")
				}
				setAdminState(user.ID, STATE_WOMAN_INFO)
				return c.Send("Годы приняты. Добавьте биографическую справку:", menuCancel, tele.ModeHTML)
			case STATE_WOMAN_INFO:
				if err := womanManager.WithDraft(user.ID, func(d *Woman) error {
					d.Info = text
					return nil
				}); err != nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Сессия истекла.")
				}
				if isAdmin(user.ID) {
					setAdminState(user.ID, STATE_WOMAN_TAGS)
					return c.Send("Биография сохранена. Добавьте теги (через запятую) или '-' чтобы пропустить:", buildCancelEditMenu(), tele.ModeHTML)
				}
				setAdminState(user.ID, STATE_WOMAN_MEDIA)
				menuFinish := buildFinishPhotoMenu()
				if !isAdmin(user.ID) {
					menuFinish = buildFinishSuggestMenu()
				}
				return c.Send("Биография сохранена. Приложите портрет (можно несколько):", menuFinish, tele.ModeHTML)
			case STATE_WOMAN_TAGS:
				if err := womanManager.WithDraft(user.ID, func(d *Woman) error {
					d.Tags = parseTagsText(text)
					return nil
				}); err != nil {
					setAdminState(user.ID, STATE_IDLE)
					return c.Reply("Сессия истекла.")
				}
				setAdminState(user.ID, STATE_WOMAN_MEDIA)
				return c.Send("Теги сохранены. Приложите портрет (можно несколько):", buildFinishPhotoMenu(), tele.ModeHTML)
			}
		}
	}
	if chat.ID == config.TargetChatID {
		statsManager.TrackMessage(c)
		if !isAdmin(user.ID) && !isWhitelisted(user.ID) {
			isSpam, reason := checkMessageText(text)
			if isSpam {
				punishUser(c, user, reason)
				return nil
			}
		}
		if gameManager != nil && gameManager.IsActive() && text != "" {
			bot := c.Bot()
			recipient := &tele.Chat{ID: chat.ID}
			u := &tele.User{ID: user.ID, FirstName: user.FirstName, Username: user.Username}
			guess := text
			safeGo("game-check", func() {
				isWin, reply, err := gameManager.CheckGuess(guess, u)
				if err != nil {
					log.Println("Game Error:", err)
				}
				if isWin {
					_, err = bot.Send(recipient, fmt.Sprintf("🎉 <b>Истина найдена!</b>\n👤 %s\n🔮 %s", u.FirstName, reply), tele.ModeHTML)
					if err != nil {
						log.Printf("⚠️ Ошибка отправки победы: %v", err)
					}
				} else if reply != "" {
					_, err = bot.Send(recipient, reply, tele.ModeHTML)
					if err != nil {
						log.Printf("⚠️ Ошибка отправки ответа: %v", err)
					}
				}
			})
		}
	}
	return nil
}

// ==========================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ (ПОДВАЛ)
// ==========================================

func getAdminState(userID int64) string {
	adminStatesMu.Lock()
	defer adminStatesMu.Unlock()
	return adminStates[userID]
}

func setAdminState(userID int64, state string) {
	adminStatesMu.Lock()
	defer adminStatesMu.Unlock()
	adminStates[userID] = state
}

func setLastShown(userID int64, womanID uint) {
	userLastShownMu.Lock()
	userLastShown[userID] = womanID
	userLastShownMu.Unlock()
}

func getLastShown(userID int64) uint {
	userLastShownMu.Lock()
	defer userLastShownMu.Unlock()
	return userLastShown[userID]
}

func tryEdit(c tele.Context, what interface{}, opts ...interface{}) error {
	err := c.Edit(what, opts...)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return c.Respond()
	}
	if err != nil {
		log.Printf("⚠️ Ошибка редактирования сообщения: %v", err)
	}
	return err
}

func sendErasMenu(c tele.Context, edit bool) error {
	menu := &tele.ReplyMarkup{}
	btnAncient := menu.Data("Античность", "era_pick_ancient")
	btnMedieval := menu.Data("Средневековье", "era_pick_medieval")
	btnEarly := menu.Data("Раннее Новое", "era_pick_earlymod")
	btnModern := menu.Data("Новое время", "era_pick_modern")
	btnCont := menu.Data("Современность", "era_pick_contemporary")
	btnCent := menu.Data("Века", "menu_centuries")
	btnBack := menu.Data("Назад", "menu_back")
	menu.Inline(
		menu.Row(btnAncient, btnMedieval),
		menu.Row(btnEarly, btnModern),
		menu.Row(btnCont),
		menu.Row(btnCent, btnBack),
	)
	msg := "🕯 <b>Эпохи</b>\nВыберите временной пласт, и Офелия покажет несколько историй."
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func resolveEra(code string) (string, int, int, bool) {
	switch code {
	case "ancient":
		return "Античность", 1, 500, true
	case "medieval":
		return "Средневековье", 500, 1500, true
	case "earlymod":
		return "Раннее Новое время", 1500, 1800, true
	case "modern":
		return "Новое время", 1800, 1950, true
	case "contemporary":
		return "Современность", 1950, 2100, true
	default:
		return "", 0, 0, false
	}
}

func sendEraPage(c tele.Context, code string, page int, edit bool) error {
	title, from, to, ok := resolveEra(code)
	if !ok {
		return c.Respond()
	}
	const limit = 8
	if page < 0 {
		page = 0
	}
	offset := page * limit
	total := womanManager.CountWomenByYearRange(from, to)
	if total == 0 {
		if edit {
			return tryEdit(c, "В этой эпохе пока нет записей.", tele.ModeHTML)
		}
		return c.Send("В этой эпохе пока нет записей.", tele.ModeHTML)
	}
	items := womanManager.ListWomenByYearRange(from, to, limit, offset)
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, w := range items {
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("user_show_%d", w.ID))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if offset > 0 {
		nav = append(nav, menu.Data("⬅ Назад", fmt.Sprintf("era_page_%s_%d", code, page-1)))
	}
	if int64(offset+limit) < total {
		nav = append(nav, menu.Data("Вперед ➜", fmt.Sprintf("era_page_%s_%d", code, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	rows = append(rows, menu.Row(menu.Data("Случайные 5", fmt.Sprintf("era_random_%s", code))))
	rows = append(rows, menu.Row(menu.Data("Века", "menu_centuries"), menu.Data("Назад", "menu_eras")))
	menu.Inline(rows...)
	msg := fmt.Sprintf("📜 <b>%s</b> — страница %d (всего %d)", title, page+1, total)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func sendCenturyPage(c tele.Context, century int, page int, edit bool) error {
	if century <= 0 {
		return c.Respond()
	}
	const limit = 8
	if page < 0 {
		page = 0
	}
	from := (century-1)*100 + 1
	to := century * 100
	offset := page * limit
	total := womanManager.CountWomenByYearRange(from, to)
	if total == 0 {
		if edit {
			return tryEdit(c, "В этом веке пока нет записей.", tele.ModeHTML)
		}
		return c.Send("В этом веке пока нет записей.", tele.ModeHTML)
	}
	items := womanManager.ListWomenByYearRange(from, to, limit, offset)
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, w := range items {
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("user_show_%d", w.ID))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if offset > 0 {
		nav = append(nav, menu.Data("⬅ Назад", fmt.Sprintf("century_page_%d_%d", century, page-1)))
	}
	if int64(offset+limit) < total {
		nav = append(nav, menu.Data("Вперед ➜", fmt.Sprintf("century_page_%d_%d", century, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	rows = append(rows, menu.Row(menu.Data("Случайные 5", fmt.Sprintf("century_random_%d", century))))
	rows = append(rows, menu.Row(menu.Data("К векам", "menu_centuries"), menu.Data("К эпохам", "menu_eras")))
	menu.Inline(rows...)
	label := roman(century)
	if label == "" {
		label = fmt.Sprintf("%d", century)
	}
	msg := fmt.Sprintf("🏛 <b>%s век</b> — страница %d (всего %d)", label, page+1, total)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func sendNoTagsPage(c tele.Context, page int, edit bool) error {
	const limit = 8
	offset := page * limit
	total := womanManager.CountWomenWithoutTags()
	if total == 0 {
		if edit {
			return tryEdit(c, "Все записи снабжены тегами.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
		}
		return c.Send("Все записи снабжены тегами.", buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
	items := womanManager.ListWomenWithoutTags(limit, offset)
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, w := range items {
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("select_edit_%d", w.ID))
		rows = append(rows, menu.Row(btn))
	}
	// Pagination
	var nav []tele.Btn
	if offset > 0 {
		nav = append(nav, menu.Data("⬅ Назад", fmt.Sprintf("admin_notags_page_%d", page-1)))
	}
	if int64(offset+limit) < total {
		nav = append(nav, menu.Data("Вперед ➜", fmt.Sprintf("admin_notags_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	rows = append(rows, menu.Row(menu.Data("В меню", "admin_back_main")))
	menu.Inline(rows...)
	msg := fmt.Sprintf("Записи без тегов: %d (стр. %d)", total, page+1)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func sendTagsPage(c tele.Context, page int, edit bool) error {
	stats := womanManager.GetTagStats()
	if len(stats) == 0 {
		return c.Send("Теги пока не сформированы.", tele.ModeHTML)
	}
	const limit = 10
	if page < 0 {
		page = 0
	}
	start := page * limit
	if start >= len(stats) {
		start = 0
		page = 0
	}
	end := start + limit
	if end > len(stats) {
		end = len(stats)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, t := range stats[start:end] {
		label := fmt.Sprintf("%s (%d)", t.Tag, t.Count)
		btn := menu.Data(label, fmt.Sprintf("tag_pick_%s", t.Tag))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if start > 0 {
		nav = append(nav, menu.Data("⬅ Назад", fmt.Sprintf("tag_page_%d", page-1)))
	}
	if end < len(stats) {
		nav = append(nav, menu.Data("Вперед ➜", fmt.Sprintf("tag_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	rows = append(rows, menu.Row(menu.Data("В меню", "menu_back")))
	menu.Inline(rows...)
	msg := fmt.Sprintf("🏷 <b>Теги</b> — страница %d", page+1)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func sendBrowseCentury(c tele.Context, page int, edit bool) error {
	centuries := womanManager.GetAvailableCenturies()
	if len(centuries) == 0 {
		return c.Reply("Эпох пока нет.", tele.ModeHTML)
	}
	pageSize := 10
	totalPages := (len(centuries) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(centuries) {
		end = len(centuries)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, cnum := range centuries[start:end] {
		label := fmt.Sprintf("%s век", roman(cnum))
		btn := menu.Data(label, fmt.Sprintf("browse_century_%d", cnum))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, menu.Data("⬅️ Назад", fmt.Sprintf("browse_centuries_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, menu.Data("Вперед ➡️", fmt.Sprintf("browse_centuries_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	menu.Inline(rows...)
	if edit {
		return tryEdit(c, "Выберите эпоху:", menu, tele.ModeHTML)
	}
	return c.Send("Выберите эпоху:", menu, tele.ModeHTML)
}

func sendBrowseFields(c tele.Context, page int, edit bool) error {
	if c.Sender() == nil {
		return nil
	}
	st, ok := getBrowseState(c.Sender().ID)
	if !ok {
		return sendBrowseCentury(c, 0, edit)
	}
	fields := womanManager.GetFieldsByYearRange(st.YearFrom, st.YearTo)
	if len(fields) == 0 {
		return c.Reply("В этой эпохе нет записей.", tele.ModeHTML)
	}
	setBrowseCache(c.Sender().ID, browseCache{Fields: fields})
	pageSize := 8
	totalPages := (len(fields) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(fields) {
		end = len(fields)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, f := range fields[start:end] {
		idx := start + i
		btn := menu.Data(f, fmt.Sprintf("browse_field_%d", idx))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, menu.Data("⬅️ Назад", fmt.Sprintf("browse_fields_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, menu.Data("Вперед ➡️", fmt.Sprintf("browse_fields_page_%d", page+1)))
	}
	nav = append(nav, menu.Data("◀️ Эпохи", "browse_back_centuries"))
	rows = append(rows, menu.Row(nav...))
	menu.Inline(rows...)
	if edit {
		return tryEdit(c, "Выберите сферу:", menu, tele.ModeHTML)
	}
	return c.Send("Выберите сферу:", menu, tele.ModeHTML)
}

func sendBrowseTags(c tele.Context, page int, edit bool) error {
	if c.Sender() == nil {
		return nil
	}
	st, ok := getBrowseState(c.Sender().ID)
	if !ok {
		return sendBrowseCentury(c, 0, edit)
	}
	filters := SearchFilters{Field: st.Field, YearFrom: st.YearFrom, YearTo: st.YearTo, PublishedOnly: true}
	tags := womanManager.GetTagStatsByFilters(filters)
	if len(tags) == 0 {
		return c.Reply("В этой сфере нет тегов.", tele.ModeHTML)
	}
	var tagList []string
	for _, t := range tags {
		tagList = append(tagList, t.Tag)
	}
	setBrowseCache(c.Sender().ID, browseCache{Tags: tagList})
	pageSize := 8
	totalPages := (len(tagList) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(tagList) {
		end = len(tagList)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, t := range tagList[start:end] {
		idx := start + i
		btn := menu.Data("#"+t, fmt.Sprintf("browse_tag_%d", idx))
		rows = append(rows, menu.Row(btn))
	}
	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, menu.Data("⬅️ Назад", fmt.Sprintf("browse_tags_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, menu.Data("Вперед ➡️", fmt.Sprintf("browse_tags_page_%d", page+1)))
	}
	nav = append(nav, menu.Data("◀️ Сферы", "browse_back_fields"))
	rows = append(rows, menu.Row(nav...))
	menu.Inline(rows...)
	if edit {
		return tryEdit(c, "Выберите тег:", menu, tele.ModeHTML)
	}
	return c.Send("Выберите тег:", menu, tele.ModeHTML)
}

func sendBrowseResults(c tele.Context, more bool) error {
	if c.Sender() == nil {
		return nil
	}
	st, ok := getBrowseState(c.Sender().ID)
	if !ok {
		return sendBrowseCentury(c, 0, false)
	}
	filters := SearchFilters{
		Field:         st.Field,
		Tags:          []string{st.Tag},
		YearFrom:      st.YearFrom,
		YearTo:        st.YearTo,
		PublishedOnly: true,
	}
	items := womanManager.GetRandomWomenByFilters(filters, 5)
	if len(items) == 0 {
		return c.Reply("Ничего не найдено.", tele.ModeHTML)
	}
	if !more {
		era := formatEra(st.YearFrom, st.YearTo)
		header := fmt.Sprintf("🔎 <b>Навигация</b>\nЭпоха: %s\nСфера: %s\nТег: #%s", era, html.EscapeString(st.Field), html.EscapeString(st.Tag))
		c.Send(header, tele.ModeHTML)
	}
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data("Еще", "browse_more"),
		menu.Data("◀️ Сферы", "browse_back_fields"),
	))
	return c.Send("Продолжить:", menu, tele.ModeHTML)
}

func handleTagPick(c tele.Context, tag string, more bool) error {
	if c.Chat() == nil {
		return nil
	}
	items := womanManager.GetWomenByTagRandom(tag, 5)
	if len(items) == 0 {
		return c.Send("По этому тегу пока пусто.", tele.ModeHTML)
	}
	if !more {
		c.Send(fmt.Sprintf("🏷 <b>%s</b> — пять историй.", tag), tele.ModeHTML)
	}
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	menu := &tele.ReplyMarkup{}
	btnMore := menu.Data("Еще 5", fmt.Sprintf("tag_more_%s", tag))
	btnBack := menu.Data("К тегам", "tag_page_0")
	menu.Inline(menu.Row(btnMore, btnBack))
	return c.Send("Продолжить или вернуться?", menu, tele.ModeHTML)
}

func startQuiz(c tele.Context, womanID uint) error {
	if c.Sender() == nil || c.Chat() == nil {
		return nil
	}
	w, err := womanManager.GetWomanByID(womanID)
	if err != nil || w == nil {
		return c.Respond()
	}
	fields := womanManager.GetUniqueFields()
	options := []string{w.Field}
	for _, f := range fields {
		if f == w.Field {
			continue
		}
		options = append(options, f)
		if len(options) >= 4 {
			break
		}
	}
	if len(options) < 2 {
		return c.Send("Для викторины нужно больше сфер.", tele.ModeHTML)
	}
	rand.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
	correct := 0
	for i, opt := range options {
		if opt == w.Field {
			correct = i
		}
	}
	quizStatesMu.Lock()
	quizStates[c.Sender().ID] = quizState{WomanID: w.ID, Options: options, Correct: correct}
	quizStatesMu.Unlock()

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, opt := range options {
		btn := menu.Data(opt, fmt.Sprintf("quiz_pick_%d", i))
		rows = append(rows, menu.Row(btn))
	}
	menu.Inline(rows...)
	return c.Send(fmt.Sprintf("🧩 <b>Викторина</b>\nК какой сфере относится <b>%s</b>?", html.EscapeString(w.Name)), menu, tele.ModeHTML)
}

func handleQuizPick(c tele.Context, idx int) error {
	if c.Sender() == nil {
		return c.Respond()
	}
	quizStatesMu.Lock()
	state, ok := quizStates[c.Sender().ID]
	if ok {
		delete(quizStates, c.Sender().ID)
	}
	quizStatesMu.Unlock()
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "Викторина устарела."})
	}
	if idx == state.Correct {
		return c.Send("🎉 Верно. Офелия склоняет голову.", tele.ModeHTML)
	}
	correct := state.Options[state.Correct]
	return c.Send(fmt.Sprintf("🥀 Неверно. Верный ответ: <b>%s</b>", html.EscapeString(correct)), tele.ModeHTML)
}

func sendFavoritesPage(c tele.Context, userID int64, page int, edit bool) error {
	const limit = 5
	if page < 0 {
		page = 0
	}
	offset := page * limit
	total := womanManager.CountFavorites(userID)
	if total == 0 {
		if edit {
			return tryEdit(c, "Избранное пока пусто.", tele.ModeHTML)
		}
		return c.Send("Избранное пока пусто.", tele.ModeHTML)
	}
	items := womanManager.ListFavorites(userID, limit, offset)
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, w := range items {
		btn := menu.Data(fmt.Sprintf("%s (%s)", w.Name, w.Field), fmt.Sprintf("user_show_%d", w.ID))
		btnDel := menu.Data("Удалить", fmt.Sprintf("fav_remove_%d", w.ID))
		rows = append(rows, menu.Row(btn, btnDel))
	}
	var nav []tele.Btn
	if offset > 0 {
		nav = append(nav, menu.Data("⬅ Назад", fmt.Sprintf("fav_page_%d", page-1)))
	}
	if int64(offset+limit) < total {
		nav = append(nav, menu.Data("Вперед ➜", fmt.Sprintf("fav_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}
	menu.Inline(rows...)
	msg := fmt.Sprintf("⭐ <b>Избранное</b> — страница %d", page+1)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func buildRecommendations(userID int64) []Woman {
	views := womanManager.GetRecentViews(userID, 30)
	if len(views) == 0 {
		return womanManager.GetRandomWomen(3)
	}
	viewedWomen := womanManager.GetWomenByIDs(views)
	tagCounts := map[string]int{}
	fieldCounts := map[string]int{}
	viewedSet := map[uint]bool{}
	for _, w := range viewedWomen {
		viewedSet[w.ID] = true
		for _, t := range w.Tags {
			tagCounts[t]++
		}
		if w.Field != "" {
			fieldCounts[w.Field]++
		}
	}
	// топ-теги
	type pair struct {
		Key   string
		Count int
	}
	var tags []pair
	for k, v := range tagCounts {
		tags = append(tags, pair{k, v})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
	var fields []pair
	for k, v := range fieldCounts {
		fields = append(fields, pair{k, v})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Count > fields[j].Count })

	// сначала по тегам
	var recs []Woman
	if len(tags) > 0 {
		f := SearchFilters{Tags: []string{tags[0].Key}, Limit: 10, PublishedOnly: true}
		candidates := womanManager.SearchWomenAdvanced(f)
		for _, c := range candidates {
			if !viewedSet[c.ID] {
				recs = append(recs, c)
				if len(recs) >= 3 {
					return recs
				}
			}
		}
	}
	// затем по сфере
	if len(fields) > 0 {
		f := SearchFilters{Field: fields[0].Key, Limit: 10, PublishedOnly: true}
		candidates := womanManager.SearchWomenAdvanced(f)
		for _, c := range candidates {
			if !viewedSet[c.ID] {
				recs = append(recs, c)
				if len(recs) >= 3 {
					return recs
				}
			}
		}
	}
	// запасной вариант
	return womanManager.GetRandomWomen(3)
}

func pickWeeklyTheme() string {
	fields := womanManager.GetUniqueFields()
	if len(fields) == 0 {
		return ""
	}
	year, week := time.Now().ISOWeek()
	idx := (year*100 + week) % len(fields)
	return fields[idx]
}

func sendCenturiesMenu(c tele.Context, edit bool) error {
	centuries := womanManager.GetAvailableCenturies()
	if len(centuries) == 0 {
		return c.Send("Архив пока без веков.", tele.ModeHTML)
	}
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	var row []tele.Btn
	for i, cnum := range centuries {
		label := roman(cnum)
		if label == "" {
			label = fmt.Sprintf("%d", cnum)
		} else {
			label = label + " век"
		}
		btn := menu.Data(label, fmt.Sprintf("century_pick_%d", cnum))
		row = append(row, btn)
		if (i+1)%2 == 0 {
			rows = append(rows, menu.Row(row...))
			row = []tele.Btn{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, menu.Row(row...))
	}
	rows = append(rows, menu.Row(menu.Data("Эпохи", "menu_eras"), menu.Data("Назад", "menu_back")))
	menu.Inline(rows...)
	msg := "🏛 <b>Века</b>\nВыберите век."
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Send(msg, menu, tele.ModeHTML)
}

func handleEraPick(c tele.Context, code string) error {
	return sendEraPage(c, code, 0, true)
}

func handleCenturyPick(c tele.Context, century int) error {
	return sendCenturyPage(c, century, 0, true)
}

func handleEraRandom(c tele.Context, code string) error {
	if c.Chat() == nil {
		return nil
	}
	title, from, to, ok := resolveEra(code)
	if !ok {
		return c.Respond()
	}
	items := womanManager.GetWomenByYearRangeRandom(from, to, 5)
	if len(items) == 0 {
		return c.Send("В этой эпохе пока нет записей.", tele.ModeHTML)
	}
	c.Send(fmt.Sprintf("📜 <b>%s</b> — пять случайных историй.", title), tele.ModeHTML)
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	return nil
}

func handleCenturyRandom(c tele.Context, century int) error {
	if c.Chat() == nil || century <= 0 {
		return nil
	}
	from := (century-1)*100 + 1
	to := century * 100
	items := womanManager.GetWomenByYearRangeRandom(from, to, 5)
	if len(items) == 0 {
		return c.Send("В этом веке пока нет записей.", tele.ModeHTML)
	}
	label := roman(century)
	if label == "" {
		label = fmt.Sprintf("%d", century)
	}
	c.Send(fmt.Sprintf("🏛 <b>%s век</b> — пять случайных историй.", label), tele.ModeHTML)
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	return nil
}

func tokenizeSearchArgs(text string) []string {
	var tokens []string
	var buf strings.Builder
	inQuote := false
	var quote rune
	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	for _, r := range text {
		switch r {
		case '"', '\'':
			if inQuote && r == quote {
				inQuote = false
				continue
			}
			if !inQuote {
				inQuote = true
				quote = r
				continue
			}
			buf.WriteRune(r)
		case ' ', '\t', '\n', '\r':
			if inQuote {
				buf.WriteRune(r)
			} else {
				flush()
			}
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	return tokens
}

func parseSearchFilters(args []string) (SearchFilters, string) {
	f := SearchFilters{Limit: 10, PublishedOnly: true}
	var free []string
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if strings.Contains(arg, ":") {
			parts := strings.SplitN(arg, ":", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			if val == "" {
				continue
			}
			switch key {
			case "name", "q", "text":
				if f.Query == "" {
					f.Query = val
				} else {
					f.Query += " " + val
				}
			case "field", "sphere":
				f.Field = val
			case "year", "years":
				from, to := parseYearRange(val)
				if from == 0 && to == 0 {
					return f, "Неверный формат года. Пример: year:1800-1900 или year:1900"
				}
				f.YearFrom = from
				f.YearTo = to
			case "century", "era":
				c, err := strconv.Atoi(val)
				if err != nil || c <= 0 {
					return f, "Неверный формат века. Пример: century:19"
				}
				f.YearFrom = (c-1)*100 + 1
				f.YearTo = c * 100
			case "tag", "tags":
				tags := parseTagsText(val)
				f.Tags = append(f.Tags, tags...)
			case "has":
				tags := parseTagsText(val)
				f.Tags = append(f.Tags, tags...)
			default:
				free = append(free, arg)
			}
		} else {
			free = append(free, arg)
		}
	}
	if len(free) > 0 {
		if f.Query == "" {
			f.Query = strings.Join(free, " ")
		} else {
			f.Query += " " + strings.Join(free, " ")
		}
	}
	if f.Query == "" && f.Field == "" && len(f.Tags) == 0 && f.YearFrom == 0 && f.YearTo == 0 {
		return f, "Задайте хотя бы один фильтр или текст запроса."
	}
	return f, ""
}

func sendStatus(c tele.Context, edit bool) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	msg := buildStatusText()
	if edit {
		return tryEdit(c, msg, buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
	return c.Send(msg, buildStaffPanelMenuForContext(c), tele.ModeHTML)
}

func buildStatusText() string {
	s, err := womanManager.GetSettings()
	if err != nil {
		log.Printf("⚠️ Ошибка чтения настроек: %v", err)
	}

	scheduleStatus := "Остановлен"
	scheduleTime := "—"
	lastRun := "—"
	if s != nil {
		scheduleTime = s.ScheduleTime
		if s.IsActive {
			scheduleStatus = "Запущен"
		}
		if !s.LastRun.IsZero() {
			lastRun = s.LastRun.Format("02.01 15:04")
		}
	}

	uptime := "—"
	if !appStartedAt.IsZero() {
		uptime = formatDuration(time.Since(appStartedAt))
	}
	gor, alloc, _, sys := runtimeStats()

	dbSize := "—"
	if info, err := os.Stat(womanManager.FilePath); err == nil {
		dbSize = formatBytes(uint64(info.Size()))
	} else {
		log.Printf("⚠️ Ошибка чтения DB: %v", err)
	}

	knownChats := len(womanManager.GetAllKnownChats())
	verifiedCount := womanManager.VerifiedCount()

	gameState := GameState{}
	if gameManager != nil {
		gameState = gameManager.Snapshot()
	}
	gameStatus := "Остановлена"
	if gameState.IsActive {
		gameStatus = "Активна"
	}
	gameMode := gameState.Mode
	if gameMode == "" {
		gameMode = "—"
	}
	gameStart := "—"
	if !gameState.StartTime.IsZero() {
		gameStart = gameState.StartTime.Format("02.01 15:04")
	}
	theme := pickWeeklyTheme()
	if theme == "" {
		theme = "—"
	}

	msg := fmt.Sprintf("🧭 <b>Диагностика Офелии</b>\n\n"+
		"⏱ Аптайм: <b>%s</b>\n"+
		"🧵 Горутин: <b>%d</b>\n"+
		"💾 Память: <b>%s</b> (alloc) | <b>%s</b> (sys)\n"+
		"📦 DB: <b>%s</b>\n"+
		"💬 Известных чатов: <b>%d</b>\n"+
		"✅ Верифицированных: <b>%d</b>\n\n"+
		"🗝 Тема недели: <b>%s</b>\n"+
		"🕰 Хронограф: <b>%s</b> | Время: <b>%s</b> | LastRun: <b>%s</b>\n"+
		"🎯 Игра: <b>%s</b> | Режим: <b>%s</b> | Старт: <b>%s</b>",
		uptime, gor, formatBytes(alloc), formatBytes(sys), dbSize, knownChats, verifiedCount,
		theme,
		scheduleStatus, scheduleTime, lastRun,
		gameStatus, gameMode, gameStart,
	)
	return msg
}

func sendAudit(c tele.Context, edit bool) error {
	if c.Sender() == nil || !isStaff(c.Sender().ID) {
		return nil
	}
	report := buildAuditReport()
	if edit {
		return tryEdit(c, report, buildStaffPanelMenuForContext(c), tele.ModeHTML)
	}
	return c.Send(report, buildStaffPanelMenuForContext(c), tele.ModeHTML)
}

func buildAuditReport() string {
	var total, published, noName, noField, noYear, noInfo int64
	var noTags, noYearRange, badYearRange, futureYears int64

	womanManager.DB.Model(&Woman{}).Count(&total)
	womanManager.DB.Model(&Woman{}).Where("is_published = ?", true).Count(&published)
	womanManager.DB.Model(&Woman{}).Where("name IS NULL OR name = ''").Count(&noName)
	womanManager.DB.Model(&Woman{}).Where("field IS NULL OR field = ''").Count(&noField)
	womanManager.DB.Model(&Woman{}).Where("year IS NULL OR year = ''").Count(&noYear)
	womanManager.DB.Model(&Woman{}).Where("info IS NULL OR info = ''").Count(&noInfo)
	noTags = womanManager.CountWomenWithoutTags()
	womanManager.DB.Model(&Woman{}).Where("year_from = 0 AND year_to = 0").Count(&noYearRange)
	womanManager.DB.Model(&Woman{}).Where("year_from > 0 AND year_to > 0 AND year_from > year_to").Count(&badYearRange)
	womanManager.DB.Model(&Woman{}).Where("year_from > 2100 OR year_to > 2100").Count(&futureYears)

	type dupRow struct {
		Name string
		Cnt  int
	}
	var dups []dupRow
	womanManager.DB.Model(&Woman{}).
		Select("name, COUNT(*) as cnt").
		Where("name <> ''").
		Group("name").
		Having("cnt > 1").
		Order("cnt desc").
		Limit(5).
		Scan(&dups)

	dupText := "—"
	if len(dups) > 0 {
		var parts []string
		for _, d := range dups {
			parts = append(parts, fmt.Sprintf("%s (%d)", html.EscapeString(shorten(d.Name, 24)), d.Cnt))
		}
		dupText = strings.Join(parts, ", ")
	}

	return fmt.Sprintf("🔎 <b>Аудит базы</b>\n\n"+
		"Всего записей: <b>%d</b>\n"+
		"Опубликовано: <b>%d</b>\n"+
		"Без тегов: <b>%d</b>\n"+
		"Без имени: <b>%d</b>\n"+
		"Без сферы: <b>%d</b>\n"+
		"Без годов: <b>%d</b>\n"+
		"Без описания: <b>%d</b>\n"+
		"Без диапазона годов: <b>%d</b>\n"+
		"Ошибочные диапазоны: <b>%d</b>\n"+
		"Будущее (>2100): <b>%d</b>\n\n"+
		"Дубликаты (топ‑5): %s",
		total, published, noTags, noName, noField, noYear, noInfo, noYearRange, badYearRange, futureYears, dupText)
}

func sendSettingsMenu(c tele.Context) error {
	s, err := womanManager.GetSettings()
	if err != nil {
		return c.Send("Ошибка БД")
	}
	statusIcon := "Остановлен"
	if s.IsActive {
		statusIcon = "Запущен"
	}
	msg := fmt.Sprintf("Настройки Хронографа\n\nСтатус: %s\nВремя оповещения: %s (серверное)", statusIcon, s.ScheduleTime)
	return tryEdit(c, msg, buildSettingsMenu(), tele.ModeHTML)
}

func sendEditMenu(c tele.Context, w *Woman) error {
	editMenu := &tele.ReplyMarkup{}
	btnEditName := editMenu.Data(fmt.Sprintf("Имя: %s", w.Name), "do_edit_name")
	btnEditYear := editMenu.Data(fmt.Sprintf("Годы: %s", w.Year), "do_edit_year")
	btnEditField := editMenu.Data(fmt.Sprintf("Сфера: %s", w.Field), "do_edit_field")
	btnEditInfo := editMenu.Data("Изменить биографию", "do_edit_info")
	btnEditTags := editMenu.Data(fmt.Sprintf("Теги: %d", len(w.Tags)), "do_edit_tags")
	btnEditMedia := editMenu.Data("Галерея", "do_edit_media")
	btnDelete := editMenu.Data("Удалить из реестра", "do_edit_delete")
	btnBack := editMenu.Data("Назад", "admin_back_main")
	editMenu.Inline(
		editMenu.Row(btnEditName),
		editMenu.Row(btnEditYear),
		editMenu.Row(btnEditField),
		editMenu.Row(btnEditInfo),
		editMenu.Row(btnEditTags),
		editMenu.Row(btnEditMedia),
		editMenu.Row(btnDelete),
		editMenu.Row(btnBack),
	)
	return tryEdit(c, fmt.Sprintf("Редактирование записи: %s", w.Name), editMenu, tele.ModeHTML)
}

func sendUserActions(c tele.Context, womanID uint) error {
	if c.Chat() == nil {
		return nil
	}
	menu := &tele.ReplyMarkup{}
	btnFav := menu.Data("⭐ В избранное", fmt.Sprintf("fav_add_%d", womanID))
	btnRel := menu.Data("Похожие", fmt.Sprintf("rel_%d", womanID))
	btnQuiz := menu.Data("Викторина", fmt.Sprintf("quiz_%d", womanID))
	menu.Inline(
		menu.Row(btnFav, btnRel),
		menu.Row(btnQuiz),
	)
	return c.Send("Выберите следующий шаг:", menu, tele.ModeHTML)
}

func sendCardToUser(c tele.Context, w *Woman, withActions bool) error {
	if w == nil || c.Chat() == nil || c.Sender() == nil {
		return nil
	}
	if err := womanManager.SendWomanCard(c.Bot(), c.Chat(), w); err != nil {
		return err
	}
	womanManager.TrackView(c.Sender().ID, w.ID)
	setLastShown(c.Sender().ID, w.ID)
	if withActions && c.Chat().Type == tele.ChatPrivate {
		return sendUserActions(c, w.ID)
	}
	return nil
}

func sendFieldSelection(c tele.Context, field string, more bool) error {
	if c.Chat() == nil {
		return nil
	}
	if strings.HasPrefix(field, "page_") {
		pstr := strings.TrimPrefix(field, "page_")
		p, _ := strconv.Atoi(pstr)
		if p < 0 {
			p = 0
		}
		return sendUserFieldsPage(c, p, true)
	}
	if field == "random" {
		w := womanManager.GetRandomWoman()
		if w == nil {
			return c.Send("Раздел пуст.")
		}
		return sendCardToUser(c, w, true)
	}
	items := womanManager.GetRandomWomenByField(field, 5)
	if len(items) == 0 {
		return c.Send("Раздел пуст.")
	}
	if !more {
		c.Send(fmt.Sprintf("🔬 <b>%s</b> — несколько голосов.", field), tele.ModeHTML)
	}
	for i, w := range items {
		_ = sendCardToUser(c, &w, i == len(items)-1)
		time.Sleep(120 * time.Millisecond)
	}
	menu := &tele.ReplyMarkup{}
	btnMore := menu.Data("Еще 5", fmt.Sprintf("field_more_%s", field))
	btnBack := menu.Data("Назад", "field_back")
	menu.Inline(menu.Row(btnMore, btnBack))
	return c.Send("Продолжить или вернуться?", menu, tele.ModeHTML)
}

func sendUserFieldsPage(c tele.Context, page int, edit bool) error {
	fields := womanManager.GetUniqueFields()
	if len(fields) == 0 {
		if edit {
			return tryEdit(c, "Архив пока пуст.", tele.ModeHTML)
		}
		return c.Reply("Архив пока пуст.", tele.ModeHTML)
	}
	pageSize := 8
	totalPages := (len(fields) + pageSize - 1) / pageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	start := page * pageSize
	end := start + pageSize
	if end > len(fields) {
		end = len(fields)
	}

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	rows = append(rows, menu.Row(menu.Data("Случайный выбор", "field_random")))

	var currentRow []tele.Btn
	for i, field := range fields[start:end] {
		idx := start + i
		cleanField := strings.TrimSpace(field)
		btn := menu.Data(cleanField, "field_"+cleanField)
		currentRow = append(currentRow, btn)
		if (idx+1)%2 == 0 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = []tele.Btn{}
		}
	}
	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	var nav []tele.Btn
	if page > 0 {
		nav = append(nav, menu.Data("⬅️ Назад", fmt.Sprintf("field_page_%d", page-1)))
	}
	if page < totalPages-1 {
		nav = append(nav, menu.Data("Вперед ➡️", fmt.Sprintf("field_page_%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, menu.Row(nav...))
	}

	menu.Inline(rows...)
	msg := fmt.Sprintf("Выберите интересующую сферу (стр. %d/%d):", page+1, totalPages)
	if edit {
		return tryEdit(c, msg, menu, tele.ModeHTML)
	}
	return c.Reply(msg, menu, tele.ModeHTML)
}

func HandleUserWoman(c tele.Context) error {
	if c.Callback() != nil {
		return sendUserFieldsPage(c, 0, true)
	}
	return sendUserFieldsPage(c, 0, false)
}
