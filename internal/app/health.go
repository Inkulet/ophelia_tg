package app

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type healthInfo struct {
	Status     string `json:"status"`
	Uptime     string `json:"uptime"`
	Goroutines int    `json:"goroutines"`
	Alloc      string `json:"alloc"`
	Sys        string `json:"sys"`
	Time       string `json:"time"`
}

func startHealthServer(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		gor, alloc, _, sys := runtimeStats()
		info := healthInfo{
			Status:     "ok",
			Uptime:     formatDuration(time.Since(appStartedAt)),
			Goroutines: gor,
			Alloc:      formatBytes(alloc),
			Sys:        formatBytes(sys),
			Time:       time.Now().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})
	log.Printf("✅ Health endpoint: %s/health", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("⚠️ Health server stopped: %v", err)
	}
}
