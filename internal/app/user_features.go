package app

import (
	"log"
	"regexp"
	"sort"
	"strings"
)

func (wm *WomanManager) LogChange(userID int64, womanID uint, field, oldVal, newVal string) {
	logEntry := ChangeLog{
		WomanID:  womanID,
		UserID:   userID,
		Field:    field,
		OldValue: shorten(oldVal, 2000),
		NewValue: shorten(newVal, 2000),
	}
	if err := wm.DB.Create(&logEntry).Error; err != nil {
		log.Printf("⚠️ Не удалось сохранить историю изменений: %v", err)
	}
}

func (wm *WomanManager) GetChangeHistory(womanID uint, limit int) []ChangeLog {
	if limit <= 0 {
		limit = 10
	}
	var rows []ChangeLog
	wm.DB.Where("woman_id = ?", womanID).Order("created_at desc").Limit(limit).Find(&rows)
	return rows
}

func (wm *WomanManager) AddFavorite(userID int64, womanID uint) error {
	f := UserFavorite{UserID: userID, WomanID: womanID}
	return wm.DB.FirstOrCreate(&f, UserFavorite{UserID: userID, WomanID: womanID}).Error
}

func (wm *WomanManager) RemoveFavorite(userID int64, womanID uint) error {
	return wm.DB.Where("user_id = ? AND woman_id = ?", userID, womanID).Delete(&UserFavorite{}).Error
}

func (wm *WomanManager) ListFavorites(userID int64, limit, offset int) []Woman {
	if limit <= 0 {
		limit = 10
	}
	var favs []UserFavorite
	wm.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Offset(offset).Find(&favs)
	if len(favs) == 0 {
		return nil
	}
	var ids []uint
	for _, f := range favs {
		ids = append(ids, f.WomanID)
	}
	var women []Woman
	wm.DB.Where("id IN ?", ids).Find(&women)
	// Сохраняем порядок
	order := map[uint]int{}
	for i, id := range ids {
		order[id] = i
	}
	sort.Slice(women, func(i, j int) bool { return order[women[i].ID] < order[women[j].ID] })
	return women
}

func (wm *WomanManager) CountFavorites(userID int64) int64 {
	var count int64
	wm.DB.Model(&UserFavorite{}).Where("user_id = ?", userID).Count(&count)
	return count
}

func (wm *WomanManager) TrackView(userID int64, womanID uint) {
	v := UserView{UserID: userID, WomanID: womanID}
	if err := wm.DB.Create(&v).Error; err != nil {
		log.Printf("⚠️ Не удалось сохранить просмотр: %v", err)
	}
}

func (wm *WomanManager) GetRecentViews(userID int64, limit int) []uint {
	if limit <= 0 {
		limit = 20
	}
	var views []UserView
	wm.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(limit).Find(&views)
	var ids []uint
	for _, v := range views {
		ids = append(ids, v.WomanID)
	}
	return ids
}

func (wm *WomanManager) CountViews(userID int64) int64 {
	var count int64
	wm.DB.Model(&UserView{}).Where("user_id = ?", userID).Count(&count)
	return count
}

func (wm *WomanManager) GetWomenByIDs(ids []uint) []Woman {
	if len(ids) == 0 {
		return nil
	}
	var women []Woman
	wm.DB.Where("id IN ?", ids).Find(&women)
	return women
}

func (wm *WomanManager) GetSubscription(userID int64) (*UserSubscription, error) {
	var sub UserSubscription
	if err := wm.DB.First(&sub, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (wm *WomanManager) SetSubscription(userID int64, active bool, timeStr string) error {
	if timeStr == "" {
		timeStr = "09:00"
	}
	sub := UserSubscription{UserID: userID}
	if err := wm.DB.FirstOrCreate(&sub, UserSubscription{UserID: userID}).Error; err != nil {
		return err
	}
	sub.IsActive = active
	sub.Time = timeStr
	return wm.DB.Save(&sub).Error
}

func (wm *WomanManager) ListActiveSubscriptions() []UserSubscription {
	var subs []UserSubscription
	wm.DB.Where("is_active = ?", true).Find(&subs)
	return subs
}

func (wm *WomanManager) UpdateSubscription(sub *UserSubscription) error {
	return wm.DB.Save(sub).Error
}

func (wm *WomanManager) GetRelatedWomen(w *Woman, limit int) []Woman {
	if w == nil || limit <= 0 {
		return nil
	}
	candidates := []Woman{}
	if len(w.Tags) > 0 {
		tmp := wm.SearchWomenAdvanced(SearchFilters{
			Tags:          w.Tags,
			Limit:         limit * 3,
			PublishedOnly: true,
		})
		candidates = append(candidates, tmp...)
	}
	if len(candidates) < limit && w.Field != "" {
		tmp := wm.SearchWomenAdvanced(SearchFilters{
			Field:         w.Field,
			Limit:         limit * 3,
			PublishedOnly: true,
		})
		candidates = append(candidates, tmp...)
	}
	seen := map[uint]bool{}
	var out []Woman
	for _, c := range candidates {
		if c.ID == w.ID {
			continue
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (wm *WomanManager) SuggestTags(w *Woman) []string {
	if w == nil {
		return nil
	}
	text := strings.ToLower(w.Info + " " + w.Field + " " + w.Name)
	suggestions := []string{}
	for key, tag := range keywordTags {
		if strings.Contains(text, key) {
			suggestions = append(suggestions, tag)
		}
	}
	// Добавляем слова сферы как теги
	for _, part := range splitRegex.Split(strings.ToLower(w.Field), -1) {
		if len([]rune(part)) >= 4 {
			suggestions = append(suggestions, part)
		}
	}
	return normalizeTags(suggestions)
}

// Ключевые слова для авто-тегов
var keywordTags = map[string]string{
	"физик":     "физика",
	"математ":   "математика",
	"поэт":      "поэзия",
	"поэтес":    "поэзия",
	"писател":   "литература",
	"биолог":    "биология",
	"астроном":  "астрономия",
	"философ":   "философия",
	"врач":      "медицина",
	"медик":     "медицина",
	"химик":     "химия",
	"юрист":     "право",
	"адвокат":   "право",
	"полит":     "политика",
	"активист":  "активизм",
	"худож":     "искусство",
	"скульптор": "искусство",
	"музык":     "музыка",
}

// Алиасы тегов -> каноническое имя
var tagAliases = map[string]string{
	"матан":  "математика",
	"матем":  "математика",
	"физ":    "физика",
	"био":    "биология",
	"лит":    "литература",
	"мед":    "медицина",
	"ист":    "история",
	"филос":  "философия",
	"псих":   "психология",
	"информ": "информатика",
	"хим":    "химия",
	"астро":  "астрономия",
	"право":  "право",
	"полит":  "политика",
	"экон":   "экономика",
	"муз":    "музыка",
}

func autoTagsIfEmpty(w *Woman) {
	if w == nil {
		return
	}
	if len(w.Tags) > 0 {
		return
	}
	w.Tags = wmAutoTags(w)
}

func wmAutoTags(w *Woman) []string {
	tags := []string{}
	text := strings.ToLower(w.Info + " " + w.Field + " " + w.Name)
	for key, tag := range keywordTags {
		if strings.Contains(text, key) {
			tags = append(tags, tag)
		}
	}
	for _, part := range splitRegex.Split(strings.ToLower(w.Field), -1) {
		if len([]rune(part)) >= 4 {
			tags = append(tags, part)
		}
	}
	return normalizeTags(tags)
}

func normalizeYearCentury(text string) (int, int) {
	text = strings.ToLower(text)
	if strings.Contains(text, "век") {
		re := regexp.MustCompile(`(\d{1,2})\s*век`)
		if m := re.FindStringSubmatch(text); len(m) == 2 {
			cent := atoiSafe(m[1])
			if cent > 0 {
				return (cent-1)*100 + 1, cent * 100
			}
		}
	}
	return 0, 0
}
