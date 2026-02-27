package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

const (
	cmsUploadsDir         = "./uploads"
	cmsMaxMultipartMemory = 32 << 20 // 32 MiB
)

var cmsAllowedMediaExtensions = map[string]struct{}{
	".jpg": {},
	".png": {},
	".mp4": {},
}

type CMSService struct {
	repo      Repository
	uploadDir string
}

func NewCMSService(repo Repository) *CMSService {
	return NewCMSServiceWithUploadDir(repo, cmsUploadsDir)
}

func NewCMSServiceWithUploadDir(repo Repository, uploadDir string) *CMSService {
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = cmsUploadsDir
	}
	return &CMSService{
		repo:      repo,
		uploadDir: uploadDir,
	}
}

func (s *CMSService) RegisterHTTPRoutes(mux *http.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc("/cms/posts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.GetPosts(w, r)
		case http.MethodPost:
			s.CreatePost(w, r)
		default:
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/cms/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.GetEvents(w, r)
	})
	mux.HandleFunc("/cms/events/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.RegisterForEvent(w, r)
	})
}

func (s *CMSService) RegisterBotHandlers(bot *tele.Bot) {
	if bot == nil {
		return
	}
	bot.Handle("/cms_post", s.HandleBotCreatePost)
	bot.Handle("/event_manage", s.HandleBotEventManage)
}

// GetPosts returns only public posts for website.
func (s *CMSService) GetPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}
	posts, err := s.repo.ListPosts(r.Context(), false)
	if err != nil {
		writeCMSError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusOK, posts)
}

// CreatePost is admin-only endpoint.
func (s *CMSService) CreatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}

	userID, err := authorizeCMSWrite(r, false)
	if err != nil {
		writeCMSError(w, http.StatusForbidden, err.Error())
		return
	}
	if !isAdmin(userID) {
		writeCMSError(w, http.StatusForbidden, "admin role is required")
		return
	}

	post, err := s.parsePostRequest(r)
	if err != nil {
		writeCMSError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.repo.CreatePost(r.Context(), post); err != nil {
		writeCMSError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusCreated, post)
}

func (s *CMSService) GetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}
	events, err := s.repo.ListEvents(r.Context())
	if err != nil {
		writeCMSError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusOK, events)
}

// RegisterForEvent registers current user in event participants.
func (s *CMSService) RegisterForEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}

	userID, err := authorizeCMSWrite(r, true)
	if err != nil {
		writeCMSError(w, http.StatusForbidden, err.Error())
		return
	}

	eventID, err := extractEventID(r)
	if err != nil {
		writeCMSError(w, http.StatusBadRequest, err.Error())
		return
	}

	err = s.repo.AddEventParticipant(r.Context(), eventID, userID)
	switch {
	case err == nil:
		writeCMSJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"eventID": eventID,
			"userID":  userID,
		})
	case errors.Is(err, ErrCMSNotFound):
		writeCMSError(w, http.StatusNotFound, "event not found")
	case errors.Is(err, ErrEventIsFull):
		writeCMSError(w, http.StatusConflict, "event is full")
	default:
		writeCMSError(w, http.StatusInternalServerError, err.Error())
	}
}

// HandleBotCreatePost parses admin command and creates post.
func (s *CMSService) HandleBotCreatePost(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return c.Reply("Недостаточно прав.")
	}
	if s.repo == nil {
		return c.Reply("CMS-репозиторий не инициализирован.")
	}

	msg := c.Message()
	if msg == nil {
		return c.Reply("Пустое сообщение.")
	}

	title, content, err := parseBotPostPayload(msg)
	if err != nil {
		return c.Reply("Формат: /cms_post <title> | <content> (можно добавить фото/документ/mp4)")
	}

	mediaPath, err := s.saveTelegramMedia(c.Bot(), msg)
	if err != nil {
		return c.Reply("Ошибка сохранения медиа: " + err.Error())
	}

	post := &Post{
		Title:     title,
		Content:   content,
		MediaPath: mediaPath,
		IsHidden:  false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.repo.CreatePost(ctx, post); err != nil {
		return c.Reply("Не удалось создать пост: " + err.Error())
	}
	return c.Reply(fmt.Sprintf("Пост создан. ID: %s", post.ID))
}

// HandleBotEventManage shows events or participant list for selected event.
func (s *CMSService) HandleBotEventManage(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return c.Reply("Недостаточно прав.")
	}
	if s.repo == nil {
		return c.Reply("CMS-репозиторий не инициализирован.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := c.Args()
	if len(args) == 0 {
		events, err := s.repo.ListEvents(ctx)
		if err != nil {
			return c.Reply("Не удалось получить события: " + err.Error())
		}
		if len(events) == 0 {
			return c.Reply("Событий пока нет.")
		}
		var sb strings.Builder
		sb.WriteString("События:\n")
		for _, event := range events {
			sb.WriteString(fmt.Sprintf("• %s | %s | %d/%d\n",
				event.ID,
				event.Date.Format("02.01.2006 15:04"),
				len(event.CurrentParticipants),
				event.MaxParticipants,
			))
		}
		sb.WriteString("\n/event_manage <event_id> — список участников")
		return c.Reply(sb.String())
	}

	eventID := strings.TrimSpace(args[0])
	event, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, ErrCMSNotFound) {
			return c.Reply("Событие не найдено.")
		}
		return c.Reply("Ошибка загрузки события: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Событие: %s\n", event.Title))
	sb.WriteString(fmt.Sprintf("ID: %s\n", event.ID))
	sb.WriteString(fmt.Sprintf("Дата: %s\n", event.Date.Format("02.01.2006 15:04")))
	sb.WriteString(fmt.Sprintf("Участники: %d/%d\n\n", len(event.CurrentParticipants), event.MaxParticipants))
	if len(event.CurrentParticipants) == 0 {
		sb.WriteString("Список пуст.")
	} else {
		for _, userID := range event.CurrentParticipants {
			sb.WriteString(fmt.Sprintf("• %d\n", userID))
		}
	}
	return c.Reply(sb.String())
}

type createPostBody struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	IsHidden bool   `json:"is_hidden"`
}

func (s *CMSService) parsePostRequest(r *http.Request) (*Post, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(cmsMaxMultipartMemory); err != nil {
			return nil, fmt.Errorf("invalid multipart payload: %w", err)
		}

		post := &Post{
			Title:    strings.TrimSpace(r.FormValue("title")),
			Content:  strings.TrimSpace(r.FormValue("content")),
			IsHidden: parseBool(r.FormValue("is_hidden")),
		}
		if post.Title == "" || post.Content == "" {
			return nil, errors.New("title and content are required")
		}

		file, header, err := r.FormFile("media")
		if err != nil && !errors.Is(err, http.ErrMissingFile) {
			return nil, fmt.Errorf("read media: %w", err)
		}
		if err == nil {
			defer file.Close()
			mediaPath, saveErr := s.saveMultipartMedia(file, header.Filename)
			if saveErr != nil {
				return nil, saveErr
			}
			post.MediaPath = mediaPath
		}

		return post, nil
	}

	var body createPostBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid json payload: %w", err)
	}
	post := &Post{
		Title:    strings.TrimSpace(body.Title),
		Content:  strings.TrimSpace(body.Content),
		IsHidden: body.IsHidden,
	}
	if post.Title == "" || post.Content == "" {
		return nil, errors.New("title and content are required")
	}
	return post, nil
}

func extractEventID(r *http.Request) (string, error) {
	if id := strings.TrimSpace(r.URL.Query().Get("event_id")); id != "" {
		return id, nil
	}
	if id := strings.TrimSpace(r.FormValue("event_id")); id != "" {
		return id, nil
	}
	var body struct {
		EventID string `json:"event_id"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && strings.TrimSpace(body.EventID) != "" {
			return strings.TrimSpace(body.EventID), nil
		}
	}
	return "", errors.New("event_id is required")
}

func authorizeCMSWrite(r *http.Request, allowSelf bool) (int64, error) {
	userID, err := extractCMSUserID(r)
	if err != nil {
		return 0, err
	}
	if hasPermission(userID, PermEdit) {
		return userID, nil
	}
	if allowSelf && userID > 0 {
		return userID, nil
	}
	return 0, errors.New("insufficient permissions")
}

func extractCMSUserID(r *http.Request) (int64, error) {
	candidates := []string{
		r.Header.Get("X-User-ID"),
		r.Header.Get("X-Telegram-User-ID"),
		r.Header.Get("X-Admin-ID"),
		r.URL.Query().Get("user_id"),
		r.FormValue("user_id"),
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return 0, errors.New("invalid user_id")
		}
		return id, nil
	}
	if id, ok := parseJSONUserID(r); ok {
		return id, nil
	}
	return 0, errors.New("user_id is required")
}

func parseJSONUserID(r *http.Request) (int64, bool) {
	if r == nil || r.Body == nil {
		return 0, false
	}

	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "application/json") {
		return 0, false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	if len(body) == 0 {
		return 0, false
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	raw, ok := payload["user_id"]
	if !ok {
		return 0, false
	}

	switch v := raw.(type) {
	case float64:
		id := int64(v)
		if float64(id) == v && id > 0 {
			return id, true
		}
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil && id > 0 {
			return id, true
		}
	}

	return 0, false
}

func writeCMSJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeCMSError(w http.ResponseWriter, status int, message string) {
	writeCMSJSON(w, status, map[string]any{
		"error": message,
	})
}

func parseBool(v string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	return err == nil && b
}

func parseBotPostPayload(msg *tele.Message) (string, string, error) {
	if msg == nil {
		return "", "", errors.New("empty message")
	}
	raw := strings.TrimSpace(msg.Payload)
	if raw == "" {
		raw = strings.TrimSpace(msg.Caption)
	}
	if raw == "" {
		raw = strings.TrimSpace(msg.Text)
		if strings.HasPrefix(raw, "/") {
			parts := strings.Fields(raw)
			if len(parts) > 0 {
				raw = strings.TrimSpace(strings.TrimPrefix(raw, parts[0]))
			}
		}
	}
	if raw == "" {
		return "", "", errors.New("empty payload")
	}

	if strings.Contains(raw, "|") {
		parts := strings.SplitN(raw, "|", 2)
		title := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])
		if title == "" || content == "" {
			return "", "", errors.New("empty title/content")
		}
		return title, content, nil
	}

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 2 {
		return "", "", errors.New("use title + content")
	}
	title := strings.TrimSpace(lines[0])
	content := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	if title == "" || content == "" {
		return "", "", errors.New("empty title/content")
	}
	return title, content, nil
}

func (s *CMSService) saveMultipartMedia(src multipart.File, fileName string) (string, error) {
	ext, err := allowedMediaExt(fileName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	targetName := uuid.NewString() + ext
	targetPath := filepath.Join(s.uploadDir, targetName)
	dst, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create media file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("save media file: %w", err)
	}
	return filepath.ToSlash(targetPath), nil
}

func (s *CMSService) saveTelegramMedia(bot *tele.Bot, msg *tele.Message) (string, error) {
	if bot == nil || msg == nil {
		return "", nil
	}

	var (
		fileRef *tele.File
		name    string
	)
	switch {
	case msg.Video != nil && msg.Video.FileID != "":
		fileRef = &msg.Video.File
		name = msg.Video.FileName
		if strings.TrimSpace(name) == "" {
			name = "video.mp4"
		}
	case msg.Document != nil && msg.Document.FileID != "":
		fileRef = &msg.Document.File
		name = msg.Document.FileName
	case msg.Photo != nil && msg.Photo.FileID != "":
		fileRef = &msg.Photo.File
		name = "photo.jpg"
	default:
		return "", nil
	}
	if fileRef == nil || strings.TrimSpace(fileRef.FileID) == "" {
		return "", nil
	}

	ext, err := allowedMediaExt(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	resolved := *fileRef
	if resolved.FilePath == "" {
		cloudFile, fileErr := bot.FileByID(fileRef.FileID)
		if fileErr != nil {
			return "", fmt.Errorf("resolve telegram file: %w", fileErr)
		}
		resolved = cloudFile
	}

	targetName := uuid.NewString() + ext
	targetPath := filepath.Join(s.uploadDir, targetName)
	if err := bot.Download(&resolved, targetPath); err != nil {
		return "", fmt.Errorf("download telegram file: %w", err)
	}
	return filepath.ToSlash(targetPath), nil
}

func allowedMediaExt(fileName string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	if ext == "" {
		return "", errors.New("file extension is required")
	}
	if _, ok := cmsAllowedMediaExtensions[ext]; !ok {
		return "", errors.New("unsupported extension (allowed: jpg, png, mp4)")
	}
	return ext, nil
}
