package app

import (
	"log"
	"strings"
)

func logModAction(userID int64, action string, targetID string, details string) {
	if womanManager == nil {
		return
	}
	act := strings.TrimSpace(action)
	if act == "" {
		act = "unknown"
	}
	entry := ModAction{
		UserID:   userID,
		Action:   act,
		TargetID: shorten(strings.TrimSpace(targetID), 64),
		Details:  shorten(strings.TrimSpace(details), 2000),
	}
	if err := womanManager.DB.Create(&entry).Error; err != nil {
		log.Printf("⚠️ Не удалось записать лог модерации: %v", err)
	}
}
