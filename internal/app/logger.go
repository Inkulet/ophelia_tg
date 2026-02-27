package app

import (
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	logFileMu    sync.Mutex
	logFile      *os.File
	errLogFile   *os.File
	appStartedAt time.Time
)

const (
	logMaxSizeMB  = 10
	logMaxBackups = 10
)

func InitLogger() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("OPHELIA ")

	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		log.Printf("⚠️ Не удалось создать директорию логов: %v", err)
	}
	_ = rotateLogsIfNeededLocked()
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("⚠️ Не удалось открыть %s: %v", logFilePath, err)
		return
	}
	errFile, err := os.OpenFile(errLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("⚠️ Не удалось открыть %s: %v", errLogPath, err)
	}
	logFile = file
	errLogFile = errFile
	log.SetOutput(newLevelWriter(logFile, errLogFile))
}

func CloseLogger() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	if errLogFile != nil {
		_ = errLogFile.Close()
		errLogFile = nil
	}
}

func markStart() {
	appStartedAt = time.Now()
}

func safeGo(name string, fn func()) {
	go func() {
		defer recoverPanic(name)
		fn()
	}()
}

func recoverPanic(name string) {
	if r := recover(); r != nil {
		log.Printf("💥 PANIC [%s]: %v\n%s", name, r, string(debug.Stack()))
	}
}

func RotateLogsIfNeeded() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	_ = rotateLogsIfNeededLocked()
}

func rotateLogsIfNeededLocked() error {
	_ = rotateIfNeeded(logFilePath, "bot", &logFile)
	_ = rotateIfNeeded(errLogPath, "errors", &errLogFile)
	log.SetOutput(newLevelWriter(logFile, errLogFile))
	return nil
}

func cleanupOldLogs(prefix, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type logEntry struct {
		name string
		mod  time.Time
	}
	var logs []logEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 8 || !strings.HasPrefix(name, prefix+"-") {
			continue
		}
		ext := filepath.Ext(name)
		if ext != ".log" && ext != ".gz" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logEntry{name: name, mod: info.ModTime()})
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].mod.After(logs[j].mod) })
	for i := logMaxBackups; i < len(logs); i++ {
		_ = os.Remove(filepath.Join(dir, logs[i].name))
	}
}

func rotateIfNeeded(path, prefix string, file **os.File) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	needsSizeRotate := info.Size() >= int64(logMaxSizeMB)*1024*1024
	needsDailyRotate := !sameDay(info.ModTime(), time.Now()) && info.Size() > 0
	if !needsSizeRotate && !needsDailyRotate {
		return nil
	}

	if file != nil && *file != nil {
		_ = (*file).Close()
		*file = nil
	}

	backupName := time.Now().Format("20060102-150405")
	dir := filepath.Dir(path)
	rotated := filepath.Join(dir, prefix+"-"+backupName+".log")
	if err := os.Rename(path, rotated); err != nil {
		return err
	}

	newFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if file != nil {
		*file = newFile
	}
	safeGo("log-compress-"+prefix, func() { compressLog(rotated) })
	cleanupOldLogs(prefix, dir)
	return nil
}

type levelWriter struct {
	out io.Writer
	err io.Writer
}

func newLevelWriter(mainFile, errFile *os.File) io.Writer {
	var writers []io.Writer
	writers = append(writers, os.Stdout)
	if mainFile != nil {
		writers = append(writers, mainFile)
	}
	out := io.MultiWriter(writers...)
	if errFile == nil {
		return out
	}
	return &levelWriter{out: out, err: errFile}
}

func (w *levelWriter) Write(p []byte) (int, error) {
	if w == nil {
		return 0, nil
	}
	_, _ = w.out.Write(p)
	line := string(p)
	if strings.Contains(line, "⚠️") || strings.Contains(line, "❌") || strings.Contains(line, "PANIC") || strings.Contains(line, "ERROR") {
		_, _ = w.err.Write(p)
	}
	return len(p), nil
}

func compressLog(path string) {
	if strings.HasSuffix(path, ".gz") {
		return
	}
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()
	outPath := path + ".gz"
	out, err := os.Create(outPath)
	if err != nil {
		return
	}
	gz := gzip.NewWriter(out)
	_, _ = io.Copy(gz, in)
	_ = gz.Close()
	_ = out.Close()
	_ = os.Remove(path)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
