package app

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

type topWoman struct {
	WomanID uint
	Name    string
	Count   int
}

func qualityScore(w *Woman) int {
	if w == nil {
		return 0
	}
	score := 0
	if len(w.Tags) > 0 {
		score++
	}
	if w.YearFrom > 0 || w.YearTo > 0 {
		score++
	}
	if len(w.MediaIDs) > 0 {
		score++
	}
	if len([]rune(w.Info)) >= 200 {
		score++
	}
	return score
}

func (wm *WomanManager) TopWomenByViews(limit int) []topWoman {
	if limit <= 0 {
		limit = 5
	}
	type row struct {
		WomanID uint
		Cnt     int
	}
	var rows []row
	wm.DB.Model(&UserView{}).
		Select("woman_id, count(*) as cnt").
		Group("woman_id").
		Order("cnt desc").
		Limit(limit).
		Scan(&rows)
	var ids []uint
	for _, r := range rows {
		ids = append(ids, r.WomanID)
	}
	women := wm.GetWomenByIDs(ids)
	nameByID := map[uint]string{}
	for _, w := range women {
		nameByID[w.ID] = w.Name
	}
	var out []topWoman
	for _, r := range rows {
		out = append(out, topWoman{WomanID: r.WomanID, Name: nameByID[r.WomanID], Count: r.Cnt})
	}
	return out
}

func (wm *WomanManager) TopWomenByFavorites(limit int) []topWoman {
	if limit <= 0 {
		limit = 5
	}
	type row struct {
		WomanID uint
		Cnt     int
	}
	var rows []row
	wm.DB.Model(&UserFavorite{}).
		Select("woman_id, count(*) as cnt").
		Group("woman_id").
		Order("cnt desc").
		Limit(limit).
		Scan(&rows)
	var ids []uint
	for _, r := range rows {
		ids = append(ids, r.WomanID)
	}
	women := wm.GetWomenByIDs(ids)
	nameByID := map[uint]string{}
	for _, w := range women {
		nameByID[w.ID] = w.Name
	}
	var out []topWoman
	for _, r := range rows {
		out = append(out, topWoman{WomanID: r.WomanID, Name: nameByID[r.WomanID], Count: r.Cnt})
	}
	return out
}

// optional helper to sort by count (used if needed)
func sortTopWomen(items []topWoman) {
	sort.Slice(items, func(i, j int) bool { return items[i].Count > items[j].Count })
}

func buildWeeklyReport() string {
	if womanManager == nil {
		return "Отчет недоступен."
	}
	var total, published, pending int64
	womanManager.DB.Model(&Woman{}).Count(&total)
	womanManager.DB.Model(&Woman{}).Where("is_published = ?", true).Count(&published)
	womanManager.DB.Model(&Woman{}).Where("is_published = ?", false).Count(&pending)

	var weekNew int64
	weekAgo := time.Now().AddDate(0, 0, -7)
	womanManager.DB.Model(&Woman{}).Where("created_at >= ?", weekAgo).Count(&weekNew)

	topViews := womanManager.TopWomenByViews(3)
	topFavs := womanManager.TopWomenByFavorites(3)

	var sb strings.Builder
	sb.WriteString("📈 <b>Еженедельный отчет</b>\n\n")
	sb.WriteString(fmt.Sprintf("Всего записей: %d\nОпубликовано: %d\nНа проверке: %d\nНовых за 7 дней: %d\n\n", total, published, pending, weekNew))
	sb.WriteString("👁 Топ просмотров:\n")
	for _, t := range topViews {
		sb.WriteString(fmt.Sprintf("• %s (%d)\n", html.EscapeString(t.Name), t.Count))
	}
	sb.WriteString("\n⭐ Топ избранного:\n")
	for _, t := range topFavs {
		sb.WriteString(fmt.Sprintf("• %s (%d)\n", html.EscapeString(t.Name), t.Count))
	}
	return sb.String()
}
