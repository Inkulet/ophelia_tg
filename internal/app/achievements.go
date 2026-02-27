package app

func getUserAchievements(userID int64) []string {
	if womanManager == nil {
		return nil
	}
	views := womanManager.CountViews(userID)
	favs := womanManager.CountFavorites(userID)
	var out []string
	switch {
	case views >= 200:
		out = append(out, "🏛 Хранитель Архива (200+ просмотров)")
	case views >= 50:
		out = append(out, "📜 Хроникер (50+ просмотров)")
	case views >= 10:
		out = append(out, "🔍 Исследователь (10+ просмотров)")
	}
	switch {
	case favs >= 20:
		out = append(out, "💎 Коллекционер (20+ избранных)")
	case favs >= 5:
		out = append(out, "📌 Собиратель (5+ избранных)")
	}
	return out
}
