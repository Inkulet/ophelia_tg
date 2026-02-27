package app

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// ==========================================
// КОНФИГУРАЦИЯ GIGACHAT
// ==========================================

const (
	GigaAuthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	GigaChatURL = "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
	Scope       = "GIGACHAT_API_PERS"
)

// ==========================================
// СТРУКТУРЫ
// ==========================================

type GlobalGameStats struct {
	Leaderboard map[int64]int    `json:"leaderboard"`
	PlayerNames map[int64]string `json:"player_names"`
	History     []RiddleHistory  `json:"history"`
}

type RiddleHistory struct {
	Date        string `json:"date"`
	Answer      string `json:"answer"`
	Description string `json:"description"`
	WinnerName  string `json:"winner_name"`
	WinnerID    int64  `json:"winner_id"`
}

type GameState struct {
	IsActive    bool
	Mode        string
	PhotoID     string
	Answer      string
	Description string // В режиме Картина - это скрытый контекст. В Цитатах/Описании - это текст загадки.
	StartTime   time.Time
}

type GameManager struct {
	mu           sync.Mutex
	State        GameState
	Stats        GlobalGameStats
	AuthKey      string
	AccessToken  string
	TokenExpires time.Time
	HttpClient   *http.Client
}

type GigaChatRequest struct {
	Model       string    `json:"model"`
	Messages    []GigaMsg `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type GigaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GigaChatResponse struct {
	Choices []struct {
		Message GigaMsg `json:"message"`
	} `json:"choices"`
}

type GigaTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

// ==========================================
// ИНИЦИАЛИЗАЦИЯ
// ==========================================

func InitGame(apiKey string) (*GameManager, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 60 * time.Second}

	gm := &GameManager{
		AuthKey:    apiKey,
		HttpClient: client,
		Stats: GlobalGameStats{
			Leaderboard: make(map[int64]int),
			PlayerNames: make(map[int64]string),
			History:     make([]RiddleHistory, 0),
		},
	}

	var initErr error
	if apiKey == "" {
		initErr = fmt.Errorf("GigaChat API ключ не задан")
	} else if err := gm.refreshToken(); err != nil {
		initErr = err
		log.Printf("⚠️ Ошибка авторизации GigaChat при старте (повторим позже): %v", err)
	}

	gm.loadStats()
	return gm, initErr
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (gm *GameManager) refreshToken() error {
	// Если токен есть и не истек, не обновляем
	if gm.AccessToken != "" && time.Now().Before(gm.TokenExpires) {
		return nil
	}
	if strings.TrimSpace(gm.AuthKey) == "" {
		return fmt.Errorf("GigaChat API ключ не задан")
	}

	payload := url.Values{}
	payload.Set("scope", Scope)

	req, err := http.NewRequest("POST", GigaAuthURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", generateUUID())
	req.Header.Set("Authorization", "Basic "+gm.AuthKey)

	resp, err := gm.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp GigaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	gm.AccessToken = tokenResp.AccessToken
	gm.TokenExpires = time.Unix(tokenResp.ExpiresAt/1000, 0).Add(-1 * time.Minute)
	return nil
}

// ==========================================
// НАСТРОЙКА ИГРЫ
// ==========================================

func (gm *GameManager) SetupGameMode(mode string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.State = GameState{IsActive: false, Mode: mode}
}

func (gm *GameManager) SetGamePhoto(fileID string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.State.PhotoID = fileID
}

func (gm *GameManager) SetGameAnswer(answer string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.State.Answer = strings.TrimSpace(answer)
}

func (gm *GameManager) SetGameContext(context string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.State.Description = context
}

// ==========================================
// СТАРТ ИГРЫ
// ==========================================

func (gm *GameManager) StartGameFromState(bot *tele.Bot, targetChatID int64) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.State.Mode == "" {
		return fmt.Errorf("режим игры не установлен")
	}

	gm.State.IsActive = true
	gm.State.StartTime = time.Now()

	targetChat := &tele.Chat{ID: targetChatID}
	var err error

	switch gm.State.Mode {
	case "painting":
		if gm.State.PhotoID != "" {
			photo := &tele.Photo{
				File:    tele.File{FileID: gm.State.PhotoID},
				Caption: "🖼 <b>Внимание, знатоки!</b>\n\nОфелия открывает глаза...\nУгадайте, что изображено на этой картине?",
			}
			_, err = bot.Send(targetChat, photo, tele.ModeHTML)
		} else {
			err = fmt.Errorf("фото не загружено")
		}

	case "mode_quotes":
		text := fmt.Sprintf("💬 <b>Чья это цитата?</b>\n\n<i>«%s»</i>\n\nУгадайте автора или произведение.", html.EscapeString(gm.State.Description))
		_, err = bot.Send(targetChat, text, tele.ModeHTML)

	case "mode_desc":
		text := fmt.Sprintf("📝 <b>Загадка от Офелии:</b>\n\n%s\n\nЧто или кто это?", html.EscapeString(gm.State.Description))
		_, err = bot.Send(targetChat, text, tele.ModeHTML)

	default:
		_, err = bot.Send(targetChat, "🎭 <b>Внимание!</b>\nЯ загадала новую загадку.", tele.ModeHTML)
	}

	if err != nil {
		log.Printf("Ошибка отправки старта: %v", err)
		return err
	}

	taskText := "<i>Вы можете задавать вопросы или предлагать ответы.\nПобедит тот, кто первым назовет верный ответ.</i>"
	bot.Send(targetChat, taskText, tele.ModeHTML)

	return nil
}

func (gm *GameManager) StopGame() {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.State.IsActive = false
}

func (gm *GameManager) IsActive() bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.State.IsActive
}

func (gm *GameManager) Snapshot() GameState {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.State
}

// ==========================================
// ПРОВЕРКА ОТВЕТА (С ГИБРИДНОЙ ЛОГИКОЙ)
// ==========================================

func (gm *GameManager) CheckGuess(userGuess string, user *tele.User) (bool, string, error) {
	gm.mu.Lock()
	// Делаем копии данных, чтобы не держать лок во время запроса
	correctAnswer := gm.State.Answer
	adminContext := gm.State.Description
	isActive := gm.State.IsActive
	currentMode := gm.State.Mode

	// 1. БЫСТРАЯ ПРОВЕРКА (БЕЗ НЕЙРОСЕТИ)
	// Если игрок ввел точный ответ (регистр не важен), засчитываем победу сразу.
	// Это решает 90% проблем с тем, что AI "тупит".
	if isActive && strings.EqualFold(strings.TrimSpace(userGuess), correctAnswer) {
		gm.mu.Unlock() // Разблокируем перед записью победы (recordWin сам возьмет лок)
		return gm.recordWin(user, correctAnswer, adminContext, "Великолепно! Абсолютно точный ответ.")
	}

	// Обновляем токен
	if err := gm.refreshToken(); err != nil {
		gm.mu.Unlock()
		return false, "⚠️ Мозг Офелии затуманен (ошибка сети)...", err
	}
	token := gm.AccessToken
	gm.mu.Unlock()

	if !isActive {
		return false, "", nil
	}

	// 2. ПРОВЕРКА ЧЕРЕЗ GIGACHAT (Для неточных ответов и синонимов)
	systemPrompt := fmt.Sprintf(`
    ТВОЯ РОЛЬ: Ты Офелия, ведущая викторины. Твой стиль: загадочный, немного меланхоличный, но дружелюбный.
    
    ИГРОВОЙ КОНТЕКСТ: "%s"
    ПРАВИЛЬНЫЙ ОТВЕТ: "%s"
    РЕЖИМ: %s
    ГИПОТЕЗА ИГРОКА: "%s"

    ИНСТРУКЦИЯ:
    1. Если Гипотеза совпадает с Правильным ответом по смыслу, является синонимом, частью имени или содержит ключевые слова -> ВЕРНИ STATUS: WIN. (НЕ БУДЬ ДУШНИЛОЙ! Если близко - засчитывай).
    2. Если Гипотеза неверна, но близка -> ВЕРНИ STATUS: HINT и дай подсказку.
    3. Если совсем мимо -> ВЕРНИ STATUS: WRONG.
    4. Если это не ответ, а просто болтовня -> ВЕРНИ STATUS: CHAT.

    ФОРМАТ ОТВЕТА:
    STATUS: [WIN | WRONG | HINT | CHAT]
    REPLY: [Твой текст]
    `, adminContext, correctAnswer, currentMode, userGuess)

	reqBody := GigaChatRequest{
		Model:       "GigaChat",
		Messages:    []GigaMsg{{Role: "user", Content: systemPrompt}},
		Temperature: 0.4, // Повышаем температуру, чтобы модель была гибче
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", GigaChatURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := gm.HttpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	var gigaResp GigaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&gigaResp); err != nil {
		return false, "", err
	}

	if len(gigaResp.Choices) == 0 {
		return false, "", nil
	}

	aiRaw := strings.TrimSpace(gigaResp.Choices[0].Message.Content)

	// ЛОГИРОВАНИЕ ДЛЯ ОТЛАДКИ (Смотрите в консоль!)
	log.Printf("🤖 GigaChat Check:\nAnswer: %s\nGuess: %s\nAI Response: %s", correctAnswer, userGuess, aiRaw)

	// Парсинг ответа
	status := "CHAT"
	reply := aiRaw

	lines := strings.Split(aiRaw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)

		if strings.HasPrefix(upper, "STATUS:") {
			// Убираем возможные звездочки (**STATUS**) или точки
			cleanStatus := strings.TrimPrefix(upper, "STATUS:")
			cleanStatus = strings.Trim(cleanStatus, " .*,!-_")
			status = cleanStatus
		} else if strings.HasPrefix(upper, "REPLY:") {
			reply = strings.TrimSpace(strings.TrimPrefix(line, "REPLY:"))
			if strings.HasPrefix(strings.ToUpper(reply), "REPLY:") {
				reply = strings.TrimSpace(reply[6:])
			}
		} else if !strings.HasPrefix(upper, "STATUS:") && line != "" {
			if reply == aiRaw {
				reply = ""
			}
			reply += " " + line
		}
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		reply = "..."
	}

	// Обработка статусов
	if strings.Contains(status, "WIN") {
		return gm.recordWin(user, correctAnswer, adminContext, reply)
	}

	if strings.Contains(status, "WRONG") {
		return false, fmt.Sprintf("🥀 %s", reply), nil
	}

	// CHAT / HINT
	return false, fmt.Sprintf("🌊 %s", reply), nil
}

// Вспомогательная функция записи победы (вынесена, чтобы вызывать и из быстрой проверки)
func (gm *GameManager) recordWin(user *tele.User, answer, context, reply string) (bool, string, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if !gm.State.IsActive {
		return false, "", nil
	}

	gm.State.IsActive = false
	gm.Stats.Leaderboard[user.ID]++

	displayName := "<Ник не задан>"
	if user.Username != "" {
		displayName = "@" + user.Username
	}
	gm.Stats.PlayerNames[user.ID] = displayName

	gm.Stats.History = append(gm.Stats.History, RiddleHistory{
		Date: time.Now().Format("02.01 15:04"), Answer: answer, Description: context, WinnerName: user.FirstName, WinnerID: user.ID,
	})
	gm.saveStats()

	return true, reply, nil
}

// ==========================================
// ТОП ИГРОКОВ
// ==========================================
type PlayerScore struct {
	ID    int64
	Name  string
	Score int
}

func (gm *GameManager) GetTopPlayers() string {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if len(gm.Stats.Leaderboard) == 0 {
		return "Пока никто не спас Офелию."
	}

	var scores []PlayerScore
	for id, score := range gm.Stats.Leaderboard {
		name, ok := gm.Stats.PlayerNames[id]
		if !ok || name == "" {
			name = fmt.Sprintf("ID %d", id)
		}
		scores = append(scores, PlayerScore{ID: id, Name: name, Score: score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	text := "🏆 <b>Топ знатоков:</b>\n\n"
	for i, p := range scores {
		if i >= 10 {
			break
		}
		medal := "•"
		if i == 0 {
			medal = "🥇"
		}
		if i == 1 {
			medal = "🥈"
		}
		if i == 2 {
			medal = "🥉"
		}
		text += fmt.Sprintf("%s <b>%s</b>: %d побед\n", medal, html.EscapeString(p.Name), p.Score)
	}
	return text
}

func (gm *GameManager) loadStats() {
	file, err := os.ReadFile(gameStatsFilePath)
	if err != nil {
		return
	}
	json.Unmarshal(file, &gm.Stats)
	if gm.Stats.Leaderboard == nil {
		gm.Stats.Leaderboard = make(map[int64]int)
	}
	if gm.Stats.PlayerNames == nil {
		gm.Stats.PlayerNames = make(map[int64]string)
	}
}

func (gm *GameManager) saveStats() {
	// Ensure directory exists
	dir := filepath.Dir(gameStatsFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("⚠️ Ошибка создания директории для статистики: %v", err)
		return
	}

	data, _ := json.MarshalIndent(gm.Stats, "", "  ")
	if err := os.WriteFile(gameStatsFilePath+".tmp", data, 0644); err != nil {
		log.Printf("⚠️ Ошибка сохранения game stats: %v", err)
		return
	}
	if err := os.Rename(gameStatsFilePath+".tmp", gameStatsFilePath); err != nil {
		log.Printf("⚠️ Ошибка сохранения game stats (rename): %v", err)
	}
}
