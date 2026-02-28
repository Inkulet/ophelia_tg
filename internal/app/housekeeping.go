package app

import (
	"log"
	"time"
)

func startHousekeeping() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cleanupRateLimits(36 * time.Hour)
		RotateLogsIfNeeded()
		monitorRuntime()
	}
}

func cleanupRateLimits(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	userLastReqMu.Lock()
	for id, t := range userLastReq {
		if t.Before(cutoff) {
			delete(userLastReq, id)
		}
	}
	userLastReqMu.Unlock()
}

var lastGoroutines int
var lastAliveLog time.Time

func monitorRuntime() {
	gor, alloc, _, sys := runtimeStats()
	if lastGoroutines > 0 && gor > lastGoroutines+300 {
		log.Printf("⚠️ Возможная утечка: goroutines выросли %d -> %d", lastGoroutines, gor)
	}
	if gor > 2000 {
		log.Printf("⚠️ Много goroutines: %d", gor)
	}
	if alloc > 600*1024*1024 {
		log.Printf("⚠️ Высокое потребление памяти: %s (sys %s)", formatBytes(alloc), formatBytes(sys))
	}
	if lastAliveLog.IsZero() || time.Since(lastAliveLog) > 6*time.Hour {
		uptime := time.Since(appStartedAt)
		log.Printf("💓 Watchdog: uptime %s, goroutines %d, mem %s", formatDuration(uptime), gor, formatBytes(alloc))
		lastAliveLog = time.Now()
	}
	lastGoroutines = gor
}
