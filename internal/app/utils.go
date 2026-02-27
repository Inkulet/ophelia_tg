package app

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

func runtimeStats() (goroutines int, alloc, totalAlloc, sys uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return runtime.NumGoroutine(), m.Alloc, m.TotalAlloc, m.Sys
}

func shorten(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func centuryFromYear(year int) int {
	if year <= 0 {
		return 0
	}
	return (year-1)/100 + 1
}

func roman(n int) string {
	if n <= 0 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var out strings.Builder
	for i := 0; i < len(vals); i++ {
		for n >= vals[i] {
			n -= vals[i]
			out.WriteString(syms[i])
		}
	}
	return out.String()
}

func formatEra(yearFrom, yearTo int) string {
	if yearFrom == 0 && yearTo == 0 {
		return ""
	}
	if yearFrom == 0 {
		yearFrom = yearTo
	}
	if yearTo == 0 {
		yearTo = yearFrom
	}
	c1 := centuryFromYear(yearFrom)
	c2 := centuryFromYear(yearTo)
	if c1 == 0 || c2 == 0 {
		return ""
	}
	if c1 == c2 {
		return fmt.Sprintf("%s век", roman(c1))
	}
	return fmt.Sprintf("%s–%s век", roman(c1), roman(c2))
}

func sendWithRetry(attempts int, baseDelay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	if baseDelay <= 0 {
		baseDelay = 500 * time.Millisecond
	}
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(baseDelay * time.Duration(1<<i))
	}
	return err
}
