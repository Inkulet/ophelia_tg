package app

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ==========================================
// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ МОДЕРАЦИИ
// ==========================================

var (
	// Списки доступа и фильтры
	whitelist []int64
	admins    []int64
	badWords  []string

	// МЬЮТЕКСЫ
	wordsMu sync.RWMutex // Защищает список badWords
	listsMu sync.RWMutex // Защищает whitelist/admins
)

// ==========================================
// РЕГУЛЯРНЫЕ ВЫРАЖЕНИЯ
// ==========================================

var (
	// Телефоны (форматы +7..., 8..., просто длинные числа с разделителями)
	phoneRegex = regexp.MustCompile(`(?:\+?\d{1,3})?[- .(:)]*\(?\d{3}\)?[- .)]*\d{3}[- .]*\d{2}[- .]*\d{2}`)

	// Номера карт
	cardRegex = regexp.MustCompile(`(?:\d[ -]*?){13,19}`)

	// Ссылки: ловит http, www, домены типа site.com, t.me и ЛЮБЫЕ упоминания через @
	linkRegex = regexp.MustCompile(`(?i)(https?://|www\.|[a-z0-9.-]+\.[a-z]{2,}|t\.me|telegram\.me|@)`)

	// Регулярка для очистки текста от знаков препинания (оставляет только Буквы и Цифры)
	// Используется для разбиения предложения на слова
	splitRegex = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// ==========================================
// ФУНКЦИИ ЗАГРУЗКИ И ПРОВЕРКИ
// ==========================================

func loadModerationLists() {
	var wl []int64
	var bw []string
	var ad []int64

	if err := loadJSON(whitelistFilePath, &wl); err != nil {
		fmt.Printf("⚠️ Файл whitelist.json не найден или поврежден: %v\n", err)
	}
	if err := loadJSON(wordsFilePath, &bw); err != nil {
		fmt.Printf("⚠️ Файл words.json не найден или поврежден: %v\n", err)
	}

	if err := loadJSON(adminFilePath, &ad); err != nil {
		fmt.Printf("⚠️ Файл admin.json не найден. Бот будет работать, но отчеты админам приходить не будут.\n")
	}

	listsMu.Lock()
	whitelist = wl
	admins = ad
	listsMu.Unlock()

	wordsMu.Lock()
	badWords = bw
	wordsMu.Unlock()
}

func checkMessageText(text string) (bool, string) {
	if text == "" {
		return false, ""
	}

	// 1. Проверка ссылок и @ (строгая)
	if linkRegex.MatchString(text) {
		return true, "🔗 Ссылка или @"
	}

	// 2. Проверка номеров телефонов
	if isPhoneSpam(text) {
		return true, "📞 Номер телефона"
	}

	// 3. Проверка карт
	if isCardSpam(text) {
		return true, "💳 Номер карты"
	}

	// 4. Проверка запрещенных слов (ТОЛЬКО ЦЕЛЫЕ СЛОВА)
	if containsBadWord(text) {
		return true, "📝 Запрещенное слово"
	}

	return false, ""
}

func checkNickname(user *tele.User) (bool, string) {
	fullName := fmt.Sprintf("%s %s %s", user.FirstName, user.LastName, user.Username)

	// Проверка на ссылки/телефоны в нике
	if linkRegex.MatchString(fullName) {
		return true, "🔗 Ссылка/@ в нике"
	}
	if isPhoneSpam(fullName) {
		return true, "📞 Телефон в нике"
	}

	// Проверка на плохие слова в нике
	if containsBadWord(fullName) {
		return true, "📝 Запрещенное слово в нике"
	}

	return false, ""
}

// containsBadWord разбивает текст на слова и ищет точное совпадение
func containsBadWord(text string) bool {
	// 1. Приводим к нижнему регистру
	lowerText := strings.ToLower(text)

	// 2. Заменяем все знаки препинания, скобки и смайлики на пробелы
	// "Привет, я блогер!" -> "привет я блогер "
	// "Читай мой блог." -> "читай мой блог "
	cleanText := splitRegex.ReplaceAllString(lowerText, " ")

	// 3. Разбиваем по пробелам на массив слов
	messageWords := strings.Fields(cleanText)

	wordsMu.RLock()
	defer wordsMu.RUnlock()

	// 4. Сравниваем каждое слово сообщения с каждым запрещенным словом
	for _, msgWord := range messageWords {
		for _, badWord := range badWords {
			if badWord == "" {
				continue
			}
			// ТОЧНОЕ СРАВНЕНИЕ
			// "блог" == "блог" -> TRUE
			// "блогер" == "блог" -> FALSE
			if msgWord == strings.ToLower(badWord) {
				return true
			}
		}
	}
	return false
}

// punishUser — удаляет сообщение и выдает варн
func punishUser(c tele.Context, user *tele.User, reason string) error {
	// Регистрируем нарушение в статистике
	count := statsManager.RegisterViolation(user.ID)

	c.Delete()
	go sendAdminReport(c.Bot(), user, "⚠️ УДАЛЕНИЕ", reason, c.Text())

	if count == 1 {
		statsManager.RegisterWarning()
		msg, _ := c.Bot().Send(c.Chat(), fmt.Sprintf("⚠️ @%s, сообщение удалено (%s). Предупреждение 1/2.", user.Username, reason))
		go func() { time.Sleep(90 * time.Second); c.Bot().Delete(msg) }()
	} else if count >= 2 {
		banUserImmediately(c, user, reason+" (x2)")
	}
	return nil
}

// banUserImmediately — банит пользователя и отправляет отчет
func banUserImmediately(c tele.Context, user *tele.User, reason string) error {
	c.Bot().Ban(c.Chat(), &tele.ChatMember{User: user})
	statsManager.RegisterBan(user.ID)

	go sendAdminReport(c.Bot(), user, "🚫 БАН", reason, "Auto-ban")
	return nil
}

func sendAdminReport(bot *tele.Bot, user *tele.User, action, reason, content string) {
	if len(content) > 300 {
		content = content[:300] + "..."
	}
	report := fmt.Sprintf("🛡 <b>%s</b>\n👤 %s (ID: %d)\n❓ %s\n📄 %s", action, user.FirstName, user.ID, reason, html.EscapeString(content))
	for _, adminID := range getAdmins() {
		if _, err := bot.Send(&tele.User{ID: adminID}, report, tele.ModeHTML); err != nil {
			log.Printf("⚠️ Не удалось отправить отчет админу %d: %v", adminID, err)
		}
	}
}

// ==========================================
// УПРАВЛЕНИЕ СПИСКАМИ
// ==========================================

func saveWords() error {
	wordsMu.RLock()
	data, _ := json.MarshalIndent(badWords, "", "  ")
	wordsMu.RUnlock()
	return atomicWrite(wordsFilePath, data)
}

func saveWhitelist() error {
	listsMu.RLock()
	data, _ := json.MarshalIndent(whitelist, "", "  ")
	listsMu.RUnlock()
	return atomicWrite(whitelistFilePath, data)
}

func atomicWrite(filename string, data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filename+".tmp", data, 0644); err != nil {
		return err
	}
	return os.Rename(filename+".tmp", filename)
}

func loadJSON(filename string, target interface{}) error {
	file, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, target)
}

// ==========================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ==========================================

func isWhitelisted(id int64) bool {
	listsMu.RLock()
	defer listsMu.RUnlock()
	for _, w := range whitelist {
		if w == id {
			return true
		}
	}
	return false
}

func listWhitelist() []int64 {
	listsMu.RLock()
	defer listsMu.RUnlock()
	out := make([]int64, len(whitelist))
	copy(out, whitelist)
	return out
}

func addWhitelist(id int64) bool {
	if id == 0 {
		return false
	}
	listsMu.Lock()
	defer listsMu.Unlock()
	for _, w := range whitelist {
		if w == id {
			return false
		}
	}
	whitelist = append(whitelist, id)
	return true
}

func removeWhitelist(id int64) bool {
	if id == 0 {
		return false
	}
	listsMu.Lock()
	defer listsMu.Unlock()
	for i, w := range whitelist {
		if w == id {
			whitelist = append(whitelist[:i], whitelist[i+1:]...)
			return true
		}
	}
	return false
}

func isAdmin(id int64) bool {
	listsMu.RLock()
	defer listsMu.RUnlock()
	for _, a := range admins {
		if a == id {
			return true
		}
	}
	return false
}

func isModerator(id int64) bool {
	if womanManager == nil {
		return false
	}
	return womanManager.IsModerator(id)
}

func isStaff(id int64) bool {
	return isAdmin(id) || isModerator(id)
}

func getAdmins() []int64 {
	listsMu.RLock()
	defer listsMu.RUnlock()
	out := make([]int64, len(admins))
	copy(out, admins)
	return out
}

func isPhoneSpam(s string) bool { return phoneRegex.MatchString(s) }
func isCardSpam(s string) bool  { return cardRegex.MatchString(s) }
