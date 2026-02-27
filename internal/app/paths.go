package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirConfigs    = "configs"
	dirModeration = "configs/moderation"
	dirData       = "data"
	dirStorage    = "storage"
	dirDB         = "storage/db"
	dirBackups    = "storage/backups"
	dirTmp        = "storage/tmp"
	dirLogs       = "logs"
)

var (
	configFilePath    = filepath.Join(dirConfigs, "config.json")
	appStatsFilePath  = filepath.Join(dirData, "stats.json")
	gameStatsFilePath = filepath.Join(dirData, "gamestats.json")

	dbFilePath       = filepath.Join(dirDB, "women.db")
	dbSHMFilePath    = dbFilePath + "-shm"
	dbWALFilePath    = dbFilePath + "-wal"
	dbTempImportPath = filepath.Join(dirTmp, "women_temp.db")
	dbBackupFilePath = filepath.Join(dirBackups, "women_backup_auto.db")

	whitelistFilePath = filepath.Join(dirModeration, "whitelist.json")
	wordsFilePath     = filepath.Join(dirModeration, "words.json")
	adminFilePath     = filepath.Join(dirModeration, "admin.json")

	logFilePath = filepath.Join(dirLogs, "bot.log")
	errLogPath  = filepath.Join(dirLogs, "errors.log")
)

func initAppLayout() {
	dirs := []string{dirConfigs, dirModeration, dirData, dirStorage, dirDB, dirBackups, dirTmp, dirLogs}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("⚠️ Не удалось создать каталог %s: %v\n", dir, err)
		}
	}

	migrateLegacyFile("config.json", configFilePath)
	migrateLegacyFile("stats.json", appStatsFilePath)
	migrateLegacyFile("gamestats.json", gameStatsFilePath)

	migrateLegacyFile("admin.json", adminFilePath)
	migrateLegacyFile("whitelist.json", whitelistFilePath)
	migrateLegacyFile("words.json", wordsFilePath)

	migrateLegacyFile("women.db", dbFilePath)
	migrateLegacyFile("women.db-shm", dbSHMFilePath)
	migrateLegacyFile("women.db-wal", dbWALFilePath)
	migrateLegacyFile("women_backup_auto.db", dbBackupFilePath)
	migrateLegacyFile("women_temp.db", dbTempImportPath)

	migrateLegacyFile("bot.log", logFilePath)
	migrateLegacyFile("errors.log", errLogPath)
	migrateLegacyLogFiles()
}

func migrateLegacyLogFiles() {
	patterns := []string{
		"bot-*.log",
		"bot-*.log.gz",
		"errors-*.log",
		"errors-*.log.gz",
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, oldPath := range matches {
			target := filepath.Join(dirLogs, filepath.Base(oldPath))
			migrateLegacyFile(oldPath, target)
		}
	}
}

func migrateLegacyFile(oldPath, newPath string) {
	info, err := os.Stat(oldPath)
	if err != nil || info.IsDir() {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		fmt.Printf("⚠️ Не удалось создать каталог для %s: %v\n", newPath, err)
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		fmt.Printf("⚠️ Не удалось переместить %s -> %s: %v\n", oldPath, newPath, err)
	}
}
