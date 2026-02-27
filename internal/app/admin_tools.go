package app

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

func exportCSV(includeAll bool) (string, error) {
	file, err := os.CreateTemp("", "women_export_*.csv")
	if err != nil {
		return "", err
	}
	writer := csv.NewWriter(file)
	_ = writer.Write([]string{"id", "name", "field", "year", "tags", "info", "published"})

	var women []Woman
	q := womanManager.DB.Model(&Woman{})
	if !includeAll {
		q = q.Where("is_published = ?", true)
	}
	if err := q.Find(&women).Error; err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	for _, w := range women {
		tags := strings.Join(w.Tags, ";")
		_ = writer.Write([]string{
			fmt.Sprintf("%d", w.ID),
			w.Name,
			w.Field,
			w.Year,
			tags,
			shorten(w.Info, 500),
			fmt.Sprintf("%v", w.IsPublished),
		})
	}
	writer.Flush()
	_ = file.Close()
	return file.Name(), nil
}

func mergeWomen(keepID, removeID uint, actorID int64) error {
	keep, err := womanManager.GetWomanByID(keepID)
	if err != nil || keep == nil {
		return fmt.Errorf("keep not found")
	}
	rem, err := womanManager.GetWomanByID(removeID)
	if err != nil || rem == nil {
		return fmt.Errorf("remove not found")
	}
	// Объединяем поля
	if keep.Field == "" {
		keep.Field = rem.Field
	}
	if keep.Year == "" {
		keep.Year = rem.Year
	}
	if keep.Info == "" {
		keep.Info = rem.Info
	}
	// Теги
	keep.Tags = normalizeTags(append(keep.Tags, rem.Tags...))
	// Медиа
	keep.MediaIDs = append(keep.MediaIDs, rem.MediaIDs...)
	// Сохраняем
	if err := womanManager.UpdateWoman(keep); err != nil {
		return err
	}
	if err := womanManager.DeleteWoman(rem.ID); err != nil {
		return err
	}
	womanManager.LogChange(actorID, keep.ID, "merge", fmt.Sprintf("merged %d", rem.ID), "ok")
	return nil
}

func parseTagCommand(text string) (string, SearchFilters, string) {
	raw := strings.TrimSpace(text)
	raw = strings.TrimPrefix(raw, "/tagadd")
	raw = strings.TrimPrefix(raw, "/tagremove")
	raw = strings.TrimSpace(raw)
	tokens := tokenizeSearchArgs(raw)
	if len(tokens) == 0 {
		return "", SearchFilters{}, "Пример: /tagadd tag:математика field:\"точные науки\""
	}
	f := SearchFilters{PublishedOnly: true, Limit: 0}
	tag := ""
	for _, tok := range tokens {
		if strings.Contains(tok, ":") {
			parts := strings.SplitN(tok, ":", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			if val == "" {
				continue
			}
			switch key {
			case "tag", "add", "remove":
				tag = strings.ToLower(val)
			case "has":
				f.Tags = append(f.Tags, parseTagsText(val)...)
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
				f.YearFrom, f.YearTo = from, to
			case "century":
				cent, _ := strconv.Atoi(val)
				if cent > 0 {
					f.YearFrom = (cent-1)*100 + 1
					f.YearTo = cent * 100
				}
			}
		} else {
			if f.Query == "" {
				f.Query = tok
			} else {
				f.Query += " " + tok
			}
		}
	}
	if tag == "" {
		return "", f, "Не указан тег. Пример: /tagadd tag:математика field:\"точные науки\""
	}
	return tag, f, ""
}

func bulkTagUpdate(tag string, filters SearchFilters, add bool, actorID int64) (int, error) {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return 0, fmt.Errorf("empty tag")
	}
	q := womanManager.buildSearchQuery(filters)
	var women []Woman
	if err := q.Find(&women).Error; err != nil {
		return 0, err
	}
	updated := 0
	for _, w := range women {
		old := strings.Join(w.Tags, ", ")
		if add {
			w.Tags = normalizeTags(append(w.Tags, tag))
		} else {
			var nt []string
			for _, t := range w.Tags {
				if strings.ToLower(t) != tag {
					nt = append(nt, t)
				}
			}
			w.Tags = normalizeTags(nt)
		}
		if err := womanManager.UpdateWoman(&w); err != nil {
			continue
		}
		newVal := strings.Join(w.Tags, ", ")
		womanManager.LogChange(actorID, w.ID, "tags", old, newVal)
		updated++
	}
	return updated, nil
}

func runMediaCheck(bot *tele.Bot, adminID int64, limit int) {
	var women []Woman
	womanManager.DB.Where("is_published = ? AND media_ids <> ''", true).Order("id desc").Limit(limit).Find(&women)
	if len(women) == 0 {
		_, _ = bot.Send(&tele.User{ID: adminID}, "Медиа для проверки не найдено.")
		return
	}
	type issue struct {
		WomanID uint
		Name    string
		FileID  string
	}
	var bad []issue
	for _, w := range women {
		for _, fid := range w.MediaIDs {
			if _, err := bot.FileByID(fid); err != nil {
				bad = append(bad, issue{WomanID: w.ID, Name: w.Name, FileID: fid})
			}
		}
	}
	if len(bad) == 0 {
		_, _ = bot.Send(&tele.User{ID: adminID}, "Проверка завершена. Битых media_id не найдено.")
		return
	}
	var sb strings.Builder
	sb.WriteString("⚠️ <b>Битые media_id</b>\n\n")
	for i, b := range bad {
		if i >= 10 {
			sb.WriteString("... и еще несколько.\n")
			break
		}
		sb.WriteString(fmt.Sprintf("• %s (ID %d)\n", b.Name, b.WomanID))
	}
	_, _ = bot.Send(&tele.User{ID: adminID}, sb.String(), tele.ModeHTML)
}

func startBroadcast(bot *tele.Bot, senderID int64, messageText string) {
	chatIDs := getBroadcastRecipients()
	if len(chatIDs) == 0 {
		_, _ = bot.Send(&tele.User{ID: senderID}, "📭 Список адресатов пуст.")
		return
	}

	_, _ = bot.Send(&tele.User{ID: senderID}, fmt.Sprintf("🚀 <b>Начинаю рассылку...</b>\nАдресатов: %d", len(chatIDs)), tele.ModeHTML)
	runHeavy("broadcast", func() {
		success, fail := 0, 0
		for _, chatID := range chatIDs {
			err := sendWithRetry(3, 500*time.Millisecond, func() error {
				_, e := bot.Send(&tele.Chat{ID: chatID}, "📢 <b>Объявление от Офелии:</b>\n\n"+messageText, tele.ModeHTML)
				return e
			})
			if err != nil {
				fail++
				log.Printf("⚠️ Ошибка рассылки в чат %d: %v", chatID, err)
			} else {
				success++
			}
			time.Sleep(50 * time.Millisecond)
		}
		_, err := bot.Send(&tele.User{ID: senderID}, fmt.Sprintf("✅ <b>Рассылка завершена.</b>\nУспешно: %d\nОшибок: %d", success, fail), tele.ModeHTML)
		if err != nil {
			log.Printf("⚠️ Не удалось отправить отчет рассылки: %v", err)
		}
		logModAction(senderID, "broadcast", "", fmt.Sprintf("success %d, fail %d", success, fail))
		_ = womanManager.DB.Create(&BroadcastLog{
			UserID:  senderID,
			Message: shorten(messageText, 500),
			Total:   len(chatIDs),
			Success: success,
			Fail:    fail,
		}).Error
	})
}

func getBroadcastRecipients() []int64 {
	wl := listWhitelist()
	if len(wl) > 0 {
		return wl
	}
	return womanManager.GetAllKnownChats()
}
