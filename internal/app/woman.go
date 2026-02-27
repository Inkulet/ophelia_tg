package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	tele "gopkg.in/telebot.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var yearRegex = regexp.MustCompile(`\d{3,4}`)

// ==========================================
// СТРУКТУРЫ ДАННЫХ
// ==========================================

type Woman struct {
	gorm.Model
	Name        string   `json:"name"`
	Field       string   `json:"field" gorm:"index"`
	Year        string   `json:"year"`
	YearFrom    int      `json:"year_from" gorm:"index"`
	YearTo      int      `json:"year_to" gorm:"index"`
	Info        string   `json:"info"`
	MediaIDs    []string `json:"media_ids" gorm:"serializer:json"`
	Tags        []string `json:"tags" gorm:"serializer:json"`
	WebImageURL string   `json:"web_image_url"`
	IsPublished bool     `json:"is_published"`
	SuggestedBy int64    `json:"suggested_by"`
}

type BotSettings struct {
	ID             uint   `gorm:"primaryKey"`
	ScheduleTime   string `gorm:"default:'09:00'"`
	IsActive       bool   `gorm:"default:false"`
	LastRun        time.Time
	TargetChatID   int64
	BackupInterval int    `gorm:"default:7"`
	ThemeActive    bool   `gorm:"default:false"`
	ThemeTime      string `gorm:"default:'10:00'"`
	ThemeWeekday   int    `gorm:"default:1"`
	ThemeLastRun   time.Time
	HealthActive   bool   `gorm:"default:false"`
	HealthTime     string `gorm:"default:'09:30'"`
	HealthLastRun  time.Time
	ReportActive   bool   `gorm:"default:false"`
	ReportTime     string `gorm:"default:'09:15'"`
	ReportWeekday  int    `gorm:"default:1"`
	ReportLastRun  time.Time
}

type BotUser struct {
	ID         int64 `gorm:"primaryKey"`
	IsVerified bool  `gorm:"default:false"`
	CreatedAt  time.Time
}

type KnownChat struct {
	ID        int64 `gorm:"primaryKey"`
	Title     string
	Username  string
	Type      string
	UpdatedAt time.Time
}

type WomanManager struct {
	DB              *gorm.DB
	FilePath        string
	Drafts          map[int64]*Woman
	Mu              sync.RWMutex
	VerifiedCache   map[int64]bool
	ChatCache       map[int64]time.Time
	ModeratorsCache map[int64]string
	FieldsCache     []string
	FieldsCacheTime time.Time
	TagsCache       []TagStat
	TagsCacheTime   time.Time
}

// ==========================================
// ИНИЦИАЛИЗАЦИЯ
// ==========================================

func NewWomanManager(file string) *WomanManager {
	wm := &WomanManager{
		FilePath:        file,
		Drafts:          make(map[int64]*Woman),
		VerifiedCache:   make(map[int64]bool),
		ChatCache:       make(map[int64]time.Time),
		ModeratorsCache: make(map[int64]string),
	}
	wm.Connect()
	return wm
}

func (wm *WomanManager) Connect() {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(wm.FilePath), 0755); err != nil {
		log.Fatalf("❌ Ошибка создания директории БД: %v", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)", wm.FilePath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		log.Fatalf("❌ Ошибка БД: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(2 * time.Hour)

	if err := db.AutoMigrate(&Woman{}, &BotSettings{}, &BotUser{}, &KnownChat{}, &UserFavorite{}, &UserView{}, &UserSubscription{}, &ChangeLog{}, &BroadcastLog{}, &Moderator{}, &ModAction{}, &Collection{}); err != nil {
		log.Printf("⚠️ Ошибка AutoMigrate: %v", err)
	}

	var settings BotSettings
	if result := db.First(&settings, 1); result.Error != nil {
		db.Create(&BotSettings{ID: 1, ScheduleTime: "10:00", IsActive: false})
	} else {
		updated := false
		if settings.ThemeTime == "" {
			settings.ThemeTime = "10:00"
			updated = true
		}
		if settings.ThemeWeekday == 0 {
			settings.ThemeWeekday = 1
			updated = true
		}
		if settings.HealthTime == "" {
			settings.HealthTime = "09:30"
			updated = true
		}
		if settings.ReportTime == "" {
			settings.ReportTime = "09:15"
			updated = true
		}
		if settings.ReportWeekday == 0 {
			settings.ReportWeekday = 1
			updated = true
		}
		if updated {
			db.Save(&settings)
		}
	}

	wm.DB = db
	log.Println("🔌 БД подключена (WAL).")

	var users []BotUser
	db.Where("is_verified = ?", true).Find(&users)
	for _, u := range users {
		wm.VerifiedCache[u.ID] = true
	}

	var mods []Moderator
	db.Find(&mods)
	for _, m := range mods {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "moderator"
		}
		wm.ModeratorsCache[m.UserID] = role
	}

	var chats []KnownChat
	db.Find(&chats)
	for _, ch := range chats {
		wm.ChatCache[ch.ID] = ch.UpdatedAt
	}

	// Обновляем YearFrom/YearTo для старых записей (лениво, партиями)
	wm.backfillYearRanges()
	// Автоматически проставляем теги для старых записей без тегов
	wm.backfillTags()
}

func (wm *WomanManager) CloseDB() error {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()
	if wm.DB == nil {
		return nil
	}
	sqlDB, err := wm.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (wm *WomanManager) Vacuum() error {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()
	return wm.DB.Exec("VACUUM").Error
}

// ==========================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ (ЧАТЫ И ЮЗЕРЫ)
// ==========================================

func (wm *WomanManager) SaveKnownChat(chat *tele.Chat) {
	if chat == nil {
		return
	}
	now := time.Now()
	wm.Mu.RLock()
	last, exists := wm.ChatCache[chat.ID]
	wm.Mu.RUnlock()
	if exists && now.Sub(last) < 12*time.Hour {
		return
	}

	kc := KnownChat{ID: chat.ID, Title: chat.Title, Type: string(chat.Type), Username: chat.Username, UpdatedAt: now}
	if kc.Title == "" {
		kc.Title = chat.Username
	}
	if kc.Title == "" {
		kc.Title = chat.FirstName + " " + chat.LastName
	}

	safeGo("save-known-chat", func() {
		if err := wm.DB.Save(&kc).Error; err == nil {
			wm.Mu.Lock()
			wm.ChatCache[chat.ID] = now
			wm.Mu.Unlock()
		} else {
			log.Printf("⚠️ Не удалось сохранить чат %d: %v", chat.ID, err)
		}
	})
}

func (wm *WomanManager) GetAllKnownChats() []int64 {
	var chats []KnownChat
	wm.DB.Find(&chats)
	var ids []int64
	for _, c := range chats {
		ids = append(ids, c.ID)
	}
	return ids
}

func (wm *WomanManager) ListKnownChats(limit, offset int) ([]KnownChat, int64) {
	if limit <= 0 {
		limit = 20
	}
	var total int64
	wm.DB.Model(&KnownChat{}).Count(&total)
	var chats []KnownChat
	wm.DB.Order("updated_at desc").Limit(limit).Offset(offset).Find(&chats)
	return chats, total
}

func (wm *WomanManager) GetKnownChat(id int64) *KnownChat {
	var chat KnownChat
	if err := wm.DB.First(&chat, "id = ?", id).Error; err != nil {
		return nil
	}
	return &chat
}

func (wm *WomanManager) IsUserVerified(userID int64) bool {
	wm.Mu.RLock()
	verified, ok := wm.VerifiedCache[userID]
	wm.Mu.RUnlock()
	return ok && verified
}

func (wm *WomanManager) VerifiedCount() int {
	wm.Mu.RLock()
	defer wm.Mu.RUnlock()
	return len(wm.VerifiedCache)
}

func (wm *WomanManager) IsModerator(userID int64) bool {
	wm.Mu.RLock()
	_, ok := wm.ModeratorsCache[userID]
	wm.Mu.RUnlock()
	return ok
}

func (wm *WomanManager) GetModeratorRole(userID int64) (string, bool) {
	wm.Mu.RLock()
	role, ok := wm.ModeratorsCache[userID]
	wm.Mu.RUnlock()
	return role, ok
}

func (wm *WomanManager) AddModerator(userID int64, role string) error {
	if strings.TrimSpace(role) == "" {
		role = "moderator"
	}
	if err := wm.DB.Save(&Moderator{UserID: userID, Role: role}).Error; err != nil {
		return err
	}
	wm.Mu.Lock()
	wm.ModeratorsCache[userID] = role
	wm.Mu.Unlock()
	return nil
}

func (wm *WomanManager) RemoveModerator(userID int64) error {
	if err := wm.DB.Delete(&Moderator{}, userID).Error; err != nil {
		return err
	}
	wm.Mu.Lock()
	delete(wm.ModeratorsCache, userID)
	wm.Mu.Unlock()
	return nil
}

func (wm *WomanManager) ListModerators() []int64 {
	wm.Mu.RLock()
	defer wm.Mu.RUnlock()
	var out []int64
	for id := range wm.ModeratorsCache {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (wm *WomanManager) ListModeratorsWithRoles() []Moderator {
	wm.Mu.RLock()
	defer wm.Mu.RUnlock()
	var out []Moderator
	for id, role := range wm.ModeratorsCache {
		out = append(out, Moderator{UserID: id, Role: role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func (wm *WomanManager) SetUserVerified(userID int64) {
	wm.Mu.Lock()
	wm.VerifiedCache[userID] = true
	wm.Mu.Unlock()
	safeGo("set-user-verified", func() {
		user := BotUser{ID: userID, IsVerified: true}
		if err := wm.DB.Save(&user).Error; err != nil {
			log.Printf("⚠️ Не удалось сохранить верификацию %d: %v", userID, err)
		}
	})
}

func (wm *WomanManager) UnsetUserVerified(userID int64) {
	wm.Mu.Lock()
	delete(wm.VerifiedCache, userID)
	wm.Mu.Unlock()
	safeGo("unset-user-verified", func() {
		if err := wm.DB.Model(&BotUser{}).Where("id = ?", userID).Update("is_verified", false).Error; err != nil {
			log.Printf("⚠️ Не удалось снять верификацию %d: %v", userID, err)
		}
	})
}

// ==========================================
// ОСНОВНАЯ ЛОГИКА
// ==========================================

func (wm *WomanManager) GetSettings() (*BotSettings, error) {
	var s BotSettings
	result := wm.DB.First(&s, 1)
	return &s, result.Error
}

func (wm *WomanManager) UpdateSettings(s *BotSettings) error {
	return wm.DB.Save(s).Error
}

func (wm *WomanManager) StartAdding(userID int64) {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()
	wm.Drafts[userID] = &Woman{MediaIDs: []string{}, Tags: []string{}, SuggestedBy: userID}
}

func (wm *WomanManager) GetDraft(userID int64) *Woman {
	wm.Mu.RLock()
	defer wm.Mu.RUnlock()
	return wm.Drafts[userID]
}

func (wm *WomanManager) WithDraft(userID int64, fn func(*Woman) error) error {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()
	draft, ok := wm.Drafts[userID]
	if !ok || draft == nil {
		return fmt.Errorf("черновик не найден")
	}
	return fn(draft)
}

func (wm *WomanManager) SaveDraft(userID int64, isPublished bool) error {
	wm.Mu.Lock()
	defer wm.Mu.Unlock()
	draft, ok := wm.Drafts[userID]
	if !ok {
		return fmt.Errorf("черновик не найден")
	}
	draft.Field = strings.TrimSpace(draft.Field)
	draft.IsPublished = isPublished
	if draft.MediaIDs == nil {
		draft.MediaIDs = []string{}
	}
	normalizeWoman(draft)
	res := wm.DB.Create(draft)
	if res.Error == nil {
		delete(wm.Drafts, userID)
		wm.FieldsCache = nil
		wm.TagsCache = nil
	}
	return res.Error
}

func (wm *WomanManager) GetPendingSuggestions() []Woman {
	var women []Woman
	wm.DB.Where("is_published = ? AND suggested_by <> 0", false).Order("created_at asc").Find(&women)
	return women
}

func (wm *WomanManager) ApproveWoman(id uint) error {
	err := wm.DB.Model(&Woman{}).Where("id = ?", id).Update("is_published", true).Error
	if err == nil {
		wm.Mu.Lock()
		wm.FieldsCache = nil
		wm.TagsCache = nil
		wm.Mu.Unlock()
	}
	return err
}

func (wm *WomanManager) CountPending() int64 {
	var count int64
	wm.DB.Model(&Woman{}).Where("is_published = ? AND suggested_by <> 0", false).Count(&count)
	return count
}

func (wm *WomanManager) SearchWomen(query string) []Woman {
	var women []Woman
	if query == "" {
		wm.DB.Where("is_published = ?", true).Limit(10).Order("id desc").Find(&women)
		return women
	}
	q := "%" + query + "%"
	qLower := "%" + strings.ToLower(query) + "%"
	wm.DB.Where("is_published = ? AND (name LIKE ? OR name LIKE ?)", true, q, qLower).Limit(10).Find(&women)
	return women
}

func (wm *WomanManager) GetWomanByID(id uint) (*Woman, error) {
	var woman Woman
	res := wm.DB.First(&woman, id)
	return &woman, res.Error
}

func (wm *WomanManager) UpdateWoman(woman *Woman) error {
	normalizeWoman(woman)
	err := wm.DB.Save(woman).Error
	if err == nil {
		wm.Mu.Lock()
		wm.FieldsCache = nil
		wm.TagsCache = nil
		wm.Mu.Unlock()
	}
	return err
}

func (wm *WomanManager) DeleteWoman(id uint) error {
	err := wm.DB.Delete(&Woman{}, id).Error
	if err == nil {
		wm.Mu.Lock()
		wm.FieldsCache = nil
		wm.TagsCache = nil
		wm.Mu.Unlock()
	}
	return err
}

func (wm *WomanManager) GetRandomWoman() *Woman {
	var woman Woman
	res := wm.DB.Where("is_published = ?", true).Order("RANDOM()").First(&woman)
	if res.Error != nil {
		return nil
	}
	return &woman
}

func (wm *WomanManager) GetWomenByField(field string) []Woman {
	var women []Woman
	wm.DB.Where("is_published = ? AND field = ?", true, strings.TrimSpace(field)).Find(&women)
	return women
}

func (wm *WomanManager) GetUniqueFields() []string {
	wm.Mu.RLock()
	if wm.FieldsCache != nil && time.Since(wm.FieldsCacheTime) < 10*time.Minute {
		defer wm.Mu.RUnlock()
		return wm.FieldsCache
	}
	wm.Mu.RUnlock()

	var fields []string
	// Сортировка важна для индексов кнопок
	wm.DB.Model(&Woman{}).Where("is_published = ?", true).Distinct("field").Order("field").Pluck("field", &fields)
	var clean []string
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			clean = append(clean, f)
		}
	}

	wm.Mu.Lock()
	wm.FieldsCache = clean
	wm.FieldsCacheTime = time.Now()
	wm.Mu.Unlock()
	return clean
}

func (wm *WomanManager) GetFieldsByYearRange(from, to int) []string {
	if from == 0 && to == 0 {
		return wm.GetUniqueFields()
	}
	if from == 0 {
		from = to
	}
	if to == 0 {
		to = from
	}
	if from > to {
		from, to = to, from
	}
	var fields []string
	wm.DB.Model(&Woman{}).
		Where("is_published = ?", true).
		Where("year_from <= ? AND year_to >= ?", to, from).
		Distinct("field").
		Order("field").
		Pluck("field", &fields)
	var clean []string
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			clean = append(clean, f)
		}
	}
	return clean
}

func (wm *WomanManager) GetTagStats() []TagStat {
	wm.Mu.RLock()
	if wm.TagsCache != nil && time.Since(wm.TagsCacheTime) < 10*time.Minute {
		defer wm.Mu.RUnlock()
		return wm.TagsCache
	}
	wm.Mu.RUnlock()

	var women []Woman
	wm.DB.Select("id", "tags").Where("is_published = ?", true).Find(&women)
	counts := map[string]int{}
	for _, w := range women {
		for _, t := range w.Tags {
			t = strings.TrimSpace(strings.ToLower(t))
			if t == "" {
				continue
			}
			counts[t]++
		}
	}
	var stats []TagStat
	for tag, cnt := range counts {
		stats = append(stats, TagStat{Tag: tag, Count: cnt})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Tag < stats[j].Tag
		}
		return stats[i].Count > stats[j].Count
	})

	wm.Mu.Lock()
	wm.TagsCache = stats
	wm.TagsCacheTime = time.Now()
	wm.Mu.Unlock()
	return stats
}

func (wm *WomanManager) GetTagStatsByFilters(f SearchFilters) []TagStat {
	q := wm.buildSearchQuery(f)
	var women []Woman
	q.Select("tags").Find(&women)
	counts := map[string]int{}
	for _, w := range women {
		for _, t := range w.Tags {
			t = strings.TrimSpace(strings.ToLower(t))
			if t == "" {
				continue
			}
			counts[t]++
		}
	}
	var stats []TagStat
	for t, c := range counts {
		stats = append(stats, TagStat{Tag: t, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Count > stats[j].Count })
	return stats
}

func (wm *WomanManager) GetWomenByTagRandom(tag string, limit int) []Woman {
	if limit <= 0 {
		return nil
	}
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return nil
	}
	var women []Woman
	like := "%\"" + tag + "\"%"
	wm.DB.Where("is_published = ? AND tags LIKE ?", true, like).
		Order("RANDOM()").Limit(limit).Find(&women)
	return women
}

func (wm *WomanManager) GetRandomWomen(limit int) []Woman {
	if limit <= 0 {
		return nil
	}
	var women []Woman
	wm.DB.Where("is_published = ?", true).Order("RANDOM()").Limit(limit).Find(&women)
	return women
}

func (wm *WomanManager) GetRandomWomenByField(field string, limit int) []Woman {
	if limit <= 0 {
		return nil
	}
	var women []Woman
	wm.DB.Where("is_published = ? AND field = ?", true, strings.TrimSpace(field)).
		Order("RANDOM()").Limit(limit).Find(&women)
	return women
}

func (wm *WomanManager) GetWomenByYearRangeRandom(from, to, limit int) []Woman {
	if limit <= 0 {
		return nil
	}
	if from > to {
		from, to = to, from
	}
	var women []Woman
	wm.DB.Where("is_published = ? AND (year_from > 0 OR year_to > 0)", true).
		Where("year_from <= ? AND year_to >= ?", to, from).
		Order("RANDOM()").Limit(limit).Find(&women)
	return women
}

func (wm *WomanManager) CountWomenByYearRange(from, to int) int64 {
	if from > to {
		from, to = to, from
	}
	var count int64
	wm.DB.Model(&Woman{}).
		Where("is_published = ? AND (year_from > 0 OR year_to > 0)", true).
		Where("year_from <= ? AND year_to >= ?", to, from).
		Count(&count)
	return count
}

func (wm *WomanManager) ListWomenByYearRange(from, to, limit, offset int) []Woman {
	if limit <= 0 {
		return nil
	}
	if from > to {
		from, to = to, from
	}
	var women []Woman
	wm.DB.Where("is_published = ? AND (year_from > 0 OR year_to > 0)", true).
		Where("year_from <= ? AND year_to >= ?", to, from).
		Order("name asc").
		Limit(limit).
		Offset(offset).
		Find(&women)
	return women
}

func (wm *WomanManager) CountWomenWithoutTags() int64 {
	var count int64
	wm.DB.Model(&Woman{}).
		Where("is_published = ? AND (tags IS NULL OR tags = '' OR tags = '[]')", true).
		Count(&count)
	return count
}

func (wm *WomanManager) ListWomenWithoutTags(limit, offset int) []Woman {
	if limit <= 0 {
		return nil
	}
	var women []Woman
	wm.DB.Where("is_published = ? AND (tags IS NULL OR tags = '' OR tags = '[]')", true).
		Order("id desc").
		Limit(limit).
		Offset(offset).
		Find(&women)
	return women
}

func (wm *WomanManager) GetAvailableCenturies() []int {
	rows, err := wm.DB.Model(&Woman{}).
		Select("year_from, year_to").
		Where("is_published = ? AND (year_from > 0 OR year_to > 0)", true).
		Rows()
	if err != nil {
		log.Printf("⚠️ Ошибка получения веков: %v", err)
		return nil
	}
	defer rows.Close()
	centuries := map[int]bool{}
	var from, to int
	for rows.Next() {
		if err := rows.Scan(&from, &to); err != nil {
			continue
		}
		if from == 0 {
			from = to
		}
		if to == 0 {
			to = from
		}
		c1 := centuryFromYear(from)
		c2 := centuryFromYear(to)
		if c1 == 0 && c2 == 0 {
			continue
		}
		if c1 == 0 {
			c1 = c2
		}
		if c2 == 0 {
			c2 = c1
		}
		if c1 > c2 {
			c1, c2 = c2, c1
		}
		for c := c1; c <= c2; c++ {
			centuries[c] = true
		}
	}
	if len(centuries) == 0 {
		return nil
	}
	var out []int
	for c := range centuries {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

type SearchFilters struct {
	Query           string
	Field           string
	Tags            []string
	YearFrom        int
	YearTo          int
	Limit           int
	PublishedOnly   bool
	UnpublishedOnly bool
}

type TagStat struct {
	Tag   string
	Count int
}

func (wm *WomanManager) SearchWomenAdvanced(f SearchFilters) []Woman {
	q := wm.buildSearchQuery(f)
	limit := f.Limit
	if limit <= 0 {
		limit = 0
	} else if limit > 20 {
		limit = 10
	}
	var women []Woman
	if limit > 0 {
		q = q.Limit(limit)
	}
	q.Order("id desc").Find(&women)
	return women
}

func (wm *WomanManager) GetRandomWomenByFilters(f SearchFilters, limit int) []Woman {
	if limit <= 0 {
		limit = 5
	}
	q := wm.buildSearchQuery(f)
	var women []Woman
	q.Order("RANDOM()").Limit(limit).Find(&women)
	return women
}

func (wm *WomanManager) buildSearchQuery(f SearchFilters) *gorm.DB {
	q := wm.DB.Model(&Woman{})
	if f.UnpublishedOnly {
		q = q.Where("is_published = ?", false)
	} else if f.PublishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if f.Query != "" {
		like := "%" + f.Query + "%"
		q = q.Where("name LIKE ? OR info LIKE ?", like, like)
	}
	if f.Field != "" {
		like := "%" + f.Field + "%"
		q = q.Where("field LIKE ?", like)
	}
	if len(f.Tags) > 0 {
		for _, t := range f.Tags {
			if t == "" {
				continue
			}
			like := "%\"" + t + "\"%"
			q = q.Where("tags LIKE ?", like)
		}
	}
	if f.YearFrom != 0 || f.YearTo != 0 {
		from := f.YearFrom
		to := f.YearTo
		if from == 0 {
			from = to
		}
		if to == 0 {
			to = from
		}
		if from > to {
			from, to = to, from
		}
		q = q.Where("year_from > 0 OR year_to > 0")
		q = q.Where("year_from <= ? AND year_to >= ?", to, from)
	}
	return q
}

func normalizeWoman(w *Woman) {
	if w == nil {
		return
	}
	w.Name = strings.TrimSpace(w.Name)
	w.Field = strings.TrimSpace(w.Field)
	w.Year = strings.TrimSpace(w.Year)
	w.Info = strings.TrimSpace(w.Info)
	if w.MediaIDs == nil {
		w.MediaIDs = []string{}
	}
	w.Tags = normalizeTags(w.Tags)
	autoTagsIfEmpty(w)
	from, to := parseYearRange(w.Year)
	w.YearFrom = from
	w.YearTo = to
}

func normalizeTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" || t == "-" {
			continue
		}
		if canon, ok := tagAliases[t]; ok {
			t = canon
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func parseTagsText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" || text == "-" {
		return []string{}
	}
	separators := []string{",", ";", "|", "/"}
	for _, sep := range separators {
		text = strings.ReplaceAll(text, sep, ",")
	}
	parts := strings.Split(text, ",")
	return normalizeTags(parts)
}

func parseYearRange(text string) (int, int) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, 0
	}
	matches := yearRegex.FindAllString(text, -1)
	if len(matches) == 0 {
		if from, to := normalizeYearCentury(text); from != 0 || to != 0 {
			return from, to
		}
		return 0, 0
	}
	if len(matches) == 1 {
		year := atoiSafe(matches[0])
		return year, year
	}
	year1 := atoiSafe(matches[0])
	year2 := atoiSafe(matches[1])
	if year1 == 0 && year2 == 0 {
		return 0, 0
	}
	if year1 > year2 {
		year1, year2 = year2, year1
	}
	return year1, year2
}

func atoiSafe(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func (wm *WomanManager) backfillYearRanges() {
	var count int64
	wm.DB.Model(&Woman{}).Where("year_from = 0 AND year_to = 0 AND year <> ''").Count(&count)
	if count == 0 {
		return
	}
	log.Printf("⛓️ Обновляю годы для %d записей...", count)
	batchSize := 200
	var women []Woman
	wm.DB.Where("year_from = 0 AND year_to = 0 AND year <> ''").FindInBatches(&women, batchSize, func(tx *gorm.DB, batch int) error {
		for _, w := range women {
			from, to := parseYearRange(w.Year)
			if from == 0 && to == 0 {
				continue
			}
			if err := tx.Model(&Woman{}).Where("id = ?", w.ID).Updates(map[string]any{
				"year_from": from,
				"year_to":   to,
			}).Error; err != nil {
				log.Printf("⚠️ Не удалось обновить годы для ID %d: %v", w.ID, err)
			}
		}
		return nil
	})
}

func (wm *WomanManager) backfillTags() {
	var count int64
	wm.DB.Model(&Woman{}).Where("tags IS NULL OR tags = '' OR tags = '[]'").Count(&count)
	if count == 0 {
		return
	}
	log.Printf("🏷️ Добавляю авто-теги для %d записей...", count)
	batchSize := 200
	var women []Woman
	wm.DB.Where("tags IS NULL OR tags = '' OR tags = '[]'").FindInBatches(&women, batchSize, func(tx *gorm.DB, batch int) error {
		for _, w := range women {
			if len(w.Tags) > 0 {
				continue
			}
			tags := wmAutoTags(&w)
			if len(tags) == 0 {
				continue
			}
			raw, err := json.Marshal(tags)
			if err != nil {
				log.Printf("⚠️ Не удалось сериализовать теги для ID %d: %v", w.ID, err)
				continue
			}
			if err := tx.Model(&Woman{}).Where("id = ?", w.ID).Update("tags", string(raw)).Error; err != nil {
				log.Printf("⚠️ Не удалось обновить теги для ID %d: %v", w.ID, err)
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------
// ОТПРАВКА СООБЩЕНИЙ С ЗАЩИТОЙ ОТ ОШИБОК
// ---------------------------------------------------------------------

// Удаляет все HTML теги для "безопасной" отправки
func removeHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

// Заменяет опасные сочетания <...> на (...)
func cleanText(text string) string {
	return strings.ReplaceAll(text, "<...>", "(...)")
}

// Главная функция отправки карточки
func (wm *WomanManager) SendWomanCard(bot *tele.Bot, recipient tele.Recipient, w *Woman) error {
	status := ""
	if !w.IsPublished {
		if w.SuggestedBy == 0 {
			status = "🗂 <b>[ЧЕРНОВИК]</b>\n"
		} else {
			status = "📝 <b>[ЗАЯВКА]</b>\n"
		}
	}

	// 1. Подготовка текстов
	safeInfo := cleanText(w.Info)
	safeName := cleanText(w.Name)
	safeField := cleanText(w.Field)
	safeYear := cleanText(w.Year)

	era := formatEra(w.YearFrom, w.YearTo)
	eraLine := ""
	if era != "" {
		eraLine = fmt.Sprintf("⏳ <b>Эпоха:</b> %s\n", era)
	}
	tagLine := ""
	if len(w.Tags) > 0 {
		tagLine = fmt.Sprintf("🏷 <b>Теги:</b> %s\n", formatTags(w.Tags, 120))
	}

	header := fmt.Sprintf("%s👩‍🎓 <b>%s</b>\n🗓 <i>%s</i>\n🔬 <b>Сфера:</b> %s\n%s%s\n",
		status, safeName, safeYear, safeField, eraLine, tagLine)
	fullCaption := header + safeInfo

	// Считаем длину "грязного" текста (с тегами).
	// Telegram считает чистый текст, но мы берем запас, чтобы не ломать теги.
	// Лимит подписи - 1024. Если с тегами < 1024, то без тегов точно влезет.
	rawLen := len([]rune(fullCaption))

	// === СЦЕНАРИЙ А: Короткое сообщение (влезает в подпись) ===
	if rawLen <= 1024 {
		// Попытка 1: Отправка с HTML
		err := sendMedia(bot, recipient, w.MediaIDs, fullCaption, tele.ModeHTML)
		if err == nil {
			return nil
		}

		// Если ошибка - логируем и пробуем без тегов
		log.Printf("⚠️ Ошибка HTML (Short): %v. Пробую Plain Text.", err)
		plainCaption := removeHTMLTags(fullCaption)
		return sendMedia(bot, recipient, w.MediaIDs, plainCaption, tele.ModeDefault)
	}

	// === СЦЕНАРИЙ Б: Длинное сообщение (разделяем) ===
	// 1. Отправляем медиа только с заголовком
	err := sendMedia(bot, recipient, w.MediaIDs, header, tele.ModeHTML)
	if err != nil {
		// Если заголовок сломался, шлем без тегов
		log.Printf("⚠️ Ошибка заголовка: %v", err)
		plainHeader := removeHTMLTags(header)
		if err := sendMedia(bot, recipient, w.MediaIDs, plainHeader, tele.ModeDefault); err != nil {
			return err
		}
	}

	// 2. Отправляем основной текст (Info) отдельно
	// Здесь лимит 4096.
	// Если текст ОЧЕНЬ длинный, просто режем по 4000 символов.
	// ВАЖНО: Если резать HTML посередине, он сломается.
	// Поэтому для длинных текстов, если они > 4096, лучше сразу слать Plain Text, чтобы не мучиться.

	infoRunes := []rune(safeInfo)
	if len(infoRunes) > 4000 {
		// Слишком длинно для одного сообщения, шлем без тегов для надежности
		for i := 0; i < len(infoRunes); i += 4000 {
			end := i + 4000
			if end > len(infoRunes) {
				end = len(infoRunes)
			}
			chunk := string(infoRunes[i:end])
			bot.Send(recipient, removeHTMLTags(chunk), tele.ModeDefault)
		}
		return nil
	}

	// Если текст нормальный (до 4096), шлем с HTML
	_, err = bot.Send(recipient, safeInfo, tele.ModeHTML)
	if err != nil {
		log.Printf("⚠️ Ошибка текста (Long): %v. Пробую Plain Text.", err)
		_, err = bot.Send(recipient, removeHTMLTags(safeInfo), tele.ModeDefault)
	}
	return err
}

// Универсальная отправка фото/альбома
func sendMedia(bot *tele.Bot, recipient tele.Recipient, mediaIDs []string, caption string, mode tele.ParseMode) error {
	if len(mediaIDs) == 0 {
		_, err := bot.Send(recipient, caption, mode)
		return err
	}

	// Одно фото
	if len(mediaIDs) == 1 {
		p := &tele.Photo{File: tele.File{FileID: mediaIDs[0]}, Caption: caption}
		_, err := bot.Send(recipient, p, mode)
		return err
	}

	// Альбом
	var album tele.Album
	for i, fid := range mediaIDs {
		if i >= 10 {
			break
		}
		p := &tele.Photo{File: tele.File{FileID: fid}}
		if i == 0 {
			p.Caption = caption
		}
		album = append(album, p)
	}
	_, err := bot.SendAlbum(recipient, album, mode)
	return err
}

func formatTags(tags []string, maxLen int) string {
	joined := strings.Join(tags, ", ")
	if maxLen <= 0 {
		return joined
	}
	r := []rune(joined)
	if len(r) > maxLen {
		return string(r[:maxLen]) + "..."
	}
	return joined
}
