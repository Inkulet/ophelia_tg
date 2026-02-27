package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	tele "gopkg.in/telebot.v3"
)

// ==========================================
// СТРУКТУРЫ ДАННЫХ
// ==========================================

type StatsManager struct {
	FilePath string
	Data     GlobalStats
	Mu       sync.RWMutex
}

// GlobalStats — единая структура для всей статистики (чат + модерация)
type GlobalStats struct {
	// --- Чат ---
	TotalMessages  int                 `json:"total_messages"`
	TotalReactions int                 `json:"total_reactions"`
	Users          map[int64]*UserStat `json:"users"`
	Posts          map[int64]*PostStat `json:"posts"`
	ActivityLog    map[string]int      `json:"activity_log"` // "2023-10-25" -> 150

	// --- Модерация ---
	DeletedMessages int           `json:"deleted_messages"`
	BannedUsers     int           `json:"banned_users"`
	WarningsGiven   int           `json:"warnings_given"`
	Violations      map[int64]int `json:"violations"` // Текущие нарушения

	LastUpdated time.Time `json:"last_updated"`
}

type UserStat struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Username      string `json:"username"`
	MsgCount      int    `json:"msg_count"`
	WordCount     int    `json:"word_count"`
	ReactionCount int    `json:"reaction_count"`
}

type PostStat struct {
	PostID       int64     `json:"post_id"`
	Preview      string    `json:"preview"`
	CommentCount int       `json:"comment_count"`
	LastActivity time.Time `json:"last_activity"`
}

// Структура для импорта истории из Telegram
type TgExport struct {
	Messages []struct {
		ID            int64  `json:"id"`
		Type          string `json:"type"`
		Date          string `json:"date"`
		FromID        string `json:"from_id"`
		From          string `json:"from"`
		ForwardedFrom string `json:"forwarded_from"`
		Text          any    `json:"text"`
		ReplyID       int64  `json:"reply_to_message_id"`
		Reactions     []struct {
			Type  string `json:"type"`
			Count int    `json:"count"`
			Emoji string `json:"emoji"`
		} `json:"reactions"`
	} `json:"messages"`
}

// ==========================================
// ИНИЦИАЛИЗАЦИЯ
// ==========================================

func NewStatsManager(file string) *StatsManager {
	sm := &StatsManager{
		FilePath: file,
		Data: GlobalStats{
			Users:       make(map[int64]*UserStat),
			Posts:       make(map[int64]*PostStat),
			ActivityLog: make(map[string]int),
			Violations:  make(map[int64]int),
		},
	}
	sm.Load()
	return sm
}

// ==========================================
// ЛОГИКА ТРЕКИНГА (ЧАТ)
// ==========================================

func (sm *StatsManager) TrackMessage(c tele.Context) {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()

	msg := c.Message()
	if msg == nil {
		return
	}
	sender := c.Sender()

	if sender == nil {
		if msg.SenderChat != nil && msg.SenderChat.Type == tele.ChatChannel {
			sm.trackPost(msg)
			sm.saveInternal()
		}
		return
	}

	if sender.IsBot && sender.ID != 777000 {
		return
	}

	// Логика постов канала
	isChannelPost := false
	if sender.ID == 777000 {
		isChannelPost = true
	} else if msg.SenderChat != nil && msg.SenderChat.Type == tele.ChatChannel {
		isChannelPost = true
	}

	if isChannelPost {
		sm.trackPost(msg)
		sm.saveInternal()
		return
	}

	// Логика пользователя
	sm.Data.TotalMessages++

	// Активность по дням
	today := time.Now().Format("2006-01-02")
	sm.Data.ActivityLog[today]++

	sm.trackUser(sender, len(msg.Text))

	// Логика реплаев на посты (комментарии)
	if msg.ReplyTo != nil {
		originalID := int64(msg.ReplyTo.ID)
		if _, exists := sm.Data.Posts[originalID]; exists {
			sm.Data.Posts[originalID].CommentCount++
			sm.Data.Posts[originalID].LastActivity = time.Now()
		} else {
			if msg.ReplyTo.Sender != nil && msg.ReplyTo.Sender.ID == 777000 {
				sm.trackPost(msg.ReplyTo)
				sm.Data.Posts[originalID].CommentCount++
			}
		}
	}

	if sm.Data.TotalMessages%10 == 0 {
		sm.saveInternal()
	}
}

func (sm *StatsManager) TrackReaction(c tele.Context) {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()

	reaction := c.Update().MessageReaction
	if reaction == nil || reaction.User == nil {
		return
	}

	user := reaction.User
	sm.Data.TotalReactions++

	if _, ok := sm.Data.Users[user.ID]; !ok {
		sm.Data.Users[user.ID] = &UserStat{ID: user.ID, Name: user.FirstName, Username: user.Username}
	}
	sm.Data.Users[user.ID].ReactionCount++

	sm.saveInternal()
}

func (sm *StatsManager) trackUser(u *tele.User, textLen int) {
	if _, ok := sm.Data.Users[u.ID]; !ok {
		sm.Data.Users[u.ID] = &UserStat{
			ID:       u.ID,
			Name:     u.FirstName,
			Username: u.Username,
		}
	}
	user := sm.Data.Users[u.ID]
	user.MsgCount++
	user.WordCount += textLen
	if u.Username != "" {
		user.Username = u.Username
	}
}

func (sm *StatsManager) trackPost(msg *tele.Message) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	runes := []rune(text)
	if len(runes) > 30 {
		text = string(runes[:30]) + "..."
	} else if len(runes) == 0 {
		text = "[Медиа]"
	}
	sm.Data.Posts[int64(msg.ID)] = &PostStat{
		PostID:       int64(msg.ID),
		Preview:      text,
		CommentCount: 0,
		LastActivity: time.Now(),
	}
}

// ==========================================
// ЛОГИКА МОДЕРАЦИИ
// ==========================================

func (sm *StatsManager) RegisterViolation(userID int64) int {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()

	sm.Data.Violations[userID]++
	sm.Data.DeletedMessages++
	sm.saveInternal()

	return sm.Data.Violations[userID]
}

func (sm *StatsManager) RegisterWarning() {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()
	sm.Data.WarningsGiven++
	sm.saveInternal()
}

func (sm *StatsManager) RegisterBan(userID int64) {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()

	sm.Data.BannedUsers++
	delete(sm.Data.Violations, userID)
	sm.saveInternal()
}

// ==========================================
// ВИЗУАЛИЗАЦИЯ И ОТЧЕТЫ
// ==========================================

// GetUserStats возвращает отформатированную статистику для конкретного юзера
func (sm *StatsManager) GetUserStats(userID int64) string {
	sm.Mu.RLock()
	defer sm.Mu.RUnlock()

	user, exists := sm.Data.Users[userID]
	if !exists {
		return "📉 Офелия еще не видела ваших сообщений в этом чате."
	}

	violations := sm.Data.Violations[userID]

	return fmt.Sprintf("👤 <b>Твой профиль:</b>\n\n"+
		"✉️ Сообщений: <b>%d</b>\n"+
		"❤️ Реакций: <b>%d</b>\n"+
		"🔡 Символов: <b>%d</b>\n"+
		"👮 Нарушений: <b>%d</b>",
		user.MsgCount, user.ReactionCount, user.WordCount, violations)
}

func (sm *StatsManager) GenerateStatsImage() ([]byte, error) {
	sm.Mu.RLock()
	defer sm.Mu.RUnlock()

	var dates []time.Time
	var values []float64

	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i)
		dateKey := d.Format("2006-01-02")
		dates = append(dates, d)
		count := float64(sm.Data.ActivityLog[dateKey])
		values = append(values, count)
	}

	graph := chart.Chart{
		Background: chart.Style{Padding: chart.Box{Top: 20, Left: 20, Right: 20, Bottom: 20}},
		Series: []chart.Series{
			chart.TimeSeries{
				Name:    "Сообщения",
				XValues: dates,
				YValues: values,
				Style:   chart.Style{StrokeColor: chart.ColorBlue, StrokeWidth: 5.0, DotColor: chart.ColorWhite, DotWidth: 4.0},
			},
		},
		XAxis:  chart.XAxis{Name: "Дни недели", ValueFormatter: chart.TimeValueFormatterWithFormat("02 Jan")},
		YAxis:  chart.YAxis{Name: "Кол-во сообщений", ValueFormatter: func(v interface{}) string { return fmt.Sprintf("%.0f", v.(float64)) }},
		Height: 400,
		Width:  800,
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	return buffer.Bytes(), err
}

func (sm *StatsManager) GetFormattedStatsText() string {
	sm.Mu.RLock()
	defer sm.Mu.RUnlock()

	type UserSorter struct{ *UserStat }
	var sortedUsers []UserSorter
	for _, u := range sm.Data.Users {
		sortedUsers = append(sortedUsers, UserSorter{u})
	}
	sort.Slice(sortedUsers, func(i, j int) bool { return sortedUsers[i].MsgCount > sortedUsers[j].MsgCount })

	type PostSorter struct{ *PostStat }
	var sortedPosts []PostSorter
	for _, p := range sm.Data.Posts {
		sortedPosts = append(sortedPosts, PostSorter{p})
	}
	sort.Slice(sortedPosts, func(i, j int) bool { return sortedPosts[i].CommentCount > sortedPosts[j].CommentCount })

	text := fmt.Sprintf("📊 <b>ОБЩАЯ СТАТИСТИКА</b>\n\n"+
		"📨 Сообщений: <b>%d</b>\n"+
		"❤️ Реакций: <b>%d</b>\n"+
		"👥 Участников: <b>%d</b>\n"+
		"📢 Постов: <b>%d</b>\n\n"+
		"👮‍♂️ <b>МОДЕРАЦИЯ</b>\n"+
		"🗑 Удалено сообщений: <b>%d</b>\n"+
		"⚠️ Выдано варнов: <b>%d</b>\n"+
		"🚫 Забанено: <b>%d</b>\n\n",
		sm.Data.TotalMessages, sm.Data.TotalReactions, len(sm.Data.Users), len(sm.Data.Posts),
		sm.Data.DeletedMessages, sm.Data.WarningsGiven, sm.Data.BannedUsers)

	text += "🏆 <b>ТОП-5 ГОВОРУНОВ:</b>\n"
	limit := 5
	if len(sortedUsers) < limit {
		limit = len(sortedUsers)
	}
	for i := 0; i < limit; i++ {
		u := sortedUsers[i]
		name := u.Name
		if u.Username != "" {
			name = "@" + u.Username
		}
		text += fmt.Sprintf("%d. <b>%s</b>: %d сообщ. | %d симп.\n", i+1, name, u.MsgCount, u.ReactionCount)
	}

	text += "\n🔥 <b>ТОП-3 ОБСУЖДАЕМЫХ ПОСТА:</b>\n"
	limit = 3
	if len(sortedPosts) < limit {
		limit = len(sortedPosts)
	}
	for i := 0; i < limit; i++ {
		p := sortedPosts[i]
		link := fmt.Sprintf("https://t.me/c/%d/%d", cleanChatID(config.TargetChatID), p.PostID)
		text += fmt.Sprintf("• <a href=\"%s\">%s</a> (💬 %d)\n", link, p.Preview, p.CommentCount)
	}

	return text
}

// ==========================================
// ИМПОРТ И УТИЛИТЫ
// ==========================================

func (sm *StatsManager) ImportFromJSON(path string) error {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var export TgExport
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("ошибка парсинга: %v", err)
	}

	log.Printf("📥 Импорт %d сообщений...", len(export.Messages))

	for _, m := range export.Messages {
		if m.Type != "message" {
			continue
		}
		for _, reaction := range m.Reactions {
			sm.Data.TotalReactions += reaction.Count
		}
		if len(m.Date) >= 10 {
			dateKey := m.Date[:10]
			sm.Data.ActivityLog[dateKey]++
		}
		isPost := false
		if strings.HasPrefix(m.FromID, "channel") || m.ForwardedFrom != "" {
			isPost = true
		}
		if isPost {
			txt := extractText(m.Text)
			sm.Data.Posts[m.ID] = &PostStat{
				PostID:       m.ID,
				Preview:      limitStr(txt, 30),
				CommentCount: 0,
				LastActivity: time.Now(),
			}
		} else {
			sm.Data.TotalMessages++
			var uid int64
			if _, err := fmt.Sscanf(m.FromID, "user%d", &uid); err == nil {
				if _, ok := sm.Data.Users[uid]; !ok {
					sm.Data.Users[uid] = &UserStat{ID: uid, Name: m.From}
				}
				sm.Data.Users[uid].MsgCount++
			}
			if m.ReplyID != 0 {
				if _, ok := sm.Data.Posts[m.ReplyID]; ok {
					sm.Data.Posts[m.ReplyID].CommentCount++
				}
			}
		}
	}
	sm.saveInternal()
	return nil
}

func (sm *StatsManager) saveInternal() {
	sm.Data.LastUpdated = time.Now()
	data, _ := json.MarshalIndent(sm.Data, "", "  ")
	if err := os.MkdirAll(filepath.Dir(sm.FilePath), 0755); err != nil {
		log.Printf("⚠️ Ошибка создания директории статистики: %v", err)
		return
	}
	if err := os.WriteFile(sm.FilePath, data, 0644); err != nil {
		log.Printf("⚠️ Ошибка сохранения статистики: %v", err)
	}
}

func (sm *StatsManager) Save() {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()
	sm.saveInternal()
}

func (sm *StatsManager) Load() {
	sm.Mu.Lock()
	defer sm.Mu.Unlock()
	file, err := os.ReadFile(sm.FilePath)
	if err == nil {
		json.Unmarshal(file, &sm.Data)
		if sm.Data.Users == nil {
			sm.Data.Users = make(map[int64]*UserStat)
		}
		if sm.Data.Posts == nil {
			sm.Data.Posts = make(map[int64]*PostStat)
		}
		if sm.Data.ActivityLog == nil {
			sm.Data.ActivityLog = make(map[string]int)
		}
		if sm.Data.Violations == nil {
			sm.Data.Violations = make(map[int64]int)
		}
	}
}

func cleanChatID(id int64) int64 {
	str := fmt.Sprintf("%d", id)
	if len(str) > 4 && str[:4] == "-100" {
		var newID int64
		fmt.Sscanf(str[4:], "%d", &newID)
		return newID
	}
	return id
}

func extractText(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		var res string
		for _, part := range val {
			switch p := part.(type) {
			case string:
				res += p
			case map[string]interface{}:
				if t, ok := p["text"].(string); ok {
					res += t
				}
			}
		}
		return res
	default:
		return ""
	}
}

func limitStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}
