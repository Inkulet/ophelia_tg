package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

const (
	cmsUploadsDir         = "./uploads"
	cmsMaxMultipartMemory = 32 << 20 // 32 MiB
	telegramNewsSourceURL = "https://t.me/s/followmylifeflow"
)

var (
	cmsAllowedMediaExtensions = map[string]struct{}{
		".jpg": {},
		".png": {},
		".mp4": {},
	}

	reChannelMessage = regexp.MustCompile(`(?s)<div class="tgme_widget_message[^"]*"[^>]*data-post="([^"]+)"[^>]*>(.*?)<div class="tgme_widget_message_footer"`)
	reChannelText    = regexp.MustCompile(`(?s)<div class="tgme_widget_message_text js-message_text"[^>]*>(.*?)</div>`)
	reChannelBGImage = regexp.MustCompile(`background-image:url\('([^']+)'\)`)
	reChannelImage   = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	reHTMLTags       = regexp.MustCompile(`(?s)<[^>]+>`)
)

type CMSService struct {
	repo      Repository
	uploadDir string
	stateMu   sync.Mutex
	states    map[int64]string
	drafts    map[int64]*cmsBotDraft
}

type ChannelPost struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	ImageURL string `json:"image_url,omitempty"`
	PostURL  string `json:"post_url"`
}

const (
	cmsCbAdminMain           = "cms_admin_main"
	cmsCbAdminMedia          = "cms_admin_media"
	cmsCbAdminHomeAbout      = "cms_admin_home_about"
	cmsCbAdminProjects       = "cms_admin_projects"
	cmsCbAdminEvents         = "cms_admin_events"
	cmsCbAdminContacts       = "cms_admin_contacts"
	cmsCbAdminBack           = "cms_admin_back"
	cmsCbSetBackground       = "cms_set_background"
	cmsCbSetAvatar           = "cms_set_avatar"
	cmsCbSetHomeDesc         = "cms_set_home_desc"
	cmsCbSetAboutText        = "cms_set_about_text"
	cmsCbSetContactEmail     = "cms_set_contact_email"
	cmsCbSetContactPhone     = "cms_set_contact_phone"
	cmsCbSetContactLocation  = "cms_set_contact_location"
	cmsCbProjectList         = "cms_project_list"
	cmsCbProjectAdd          = "cms_project_add"
	cmsCbProjectEdit         = "cms_project_edit"
	cmsCbProjectDeleteMenu   = "cms_project_delete_menu"
	cmsCbEventList           = "cms_event_list"
	cmsCbEventAdd            = "cms_event_add"
	cmsCbEventEdit           = "cms_event_edit"
	cmsCbEventDeleteMenu     = "cms_event_delete_menu"
	cmsCbProjectPickPrefix   = "cms_project_pick_"
	cmsCbProjectFieldPrefix  = "cms_project_field_"
	cmsCbProjectDeletePrefix = "cms_project_delete_"
	cmsCbEventPickPrefix     = "cms_event_pick_"
	cmsCbEventFieldPrefix    = "cms_event_field_"
	cmsCbEventDeletePrefix   = "cms_event_delete_"
)

const (
	cmsStateIdle = ""

	cmsStateSetBackgroundMedia = "cms_set_background_media"
	cmsStateSetAvatarMedia     = "cms_set_avatar_media"
	cmsStateSetHomeDesc        = "cms_set_home_desc"
	cmsStateSetAboutText       = "cms_set_about_text"
	cmsStateSetContactEmail    = "cms_set_contact_email"
	cmsStateSetContactPhone    = "cms_set_contact_phone"
	cmsStateSetContactLocation = "cms_set_contact_location"

	cmsStateProjectCreateTitle    = "cms_project_create_title"
	cmsStateProjectCreateShort    = "cms_project_create_short"
	cmsStateProjectCreateDetailed = "cms_project_create_detailed"
	cmsStateProjectCreateMedia    = "cms_project_create_media"
	cmsStateProjectEditValue      = "cms_project_edit_value"

	cmsStateEventCreatePayload = "cms_event_create_payload"
	cmsStateEventEditValue     = "cms_event_edit_value"
)

type cmsBotDraft struct {
	ProjectID  string
	Project    Project
	EventID    string
	EventField string
	Field      string
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
		states:    make(map[int64]string),
		drafts:    make(map[int64]*cmsBotDraft),
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
	mux.HandleFunc("/cms/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.GetSettings(w, r)
	})
	mux.HandleFunc("/cms/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.GetProjects(w, r)
	})
	mux.HandleFunc("/cms/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.GetEvents(w, r)
	})
	mux.HandleFunc("/cms/news", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.GetNews(w, r)
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
	bot.Handle("/cms_event_add", s.HandleBotEventAdd)
	bot.Handle("/cms_post_del", s.HandleBotPostDelete)
}

func (s *CMSService) HandleBotSiteAdminMenu(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return c.Reply("Недостаточно прав.")
	}
	s.setState(c.Sender().ID, cmsStateIdle)
	s.resetDraft(c.Sender().ID)
	return s.renderMenu(c, false, "🛠 <b>Управление Сайтом</b>\nВыберите категорию:", s.buildSiteAdminMenu())
}

func (s *CMSService) HandleBotCMSCallback(c tele.Context, data string) (bool, error) {
	if !strings.HasPrefix(data, "cms_") {
		return false, nil
	}
	if c.Sender() == nil {
		return true, nil
	}
	userID := c.Sender().ID
	if !isAdmin(userID) {
		return true, tryEdit(c, "Недостаточно прав.", buildMainMenu(userID), tele.ModeHTML)
	}
	if s.repo == nil {
		return true, tryEdit(c, "CMS-репозиторий не инициализирован.", tele.ModeHTML)
	}

	switch data {
	case cmsCbAdminMain:
		s.setState(userID, cmsStateIdle)
		return true, s.renderMenu(c, true, "🛠 <b>Управление Сайтом</b>\nВыберите категорию:", s.buildSiteAdminMenu())
	case cmsCbAdminMedia:
		return true, s.renderMenu(c, true, "🖼 <b>Фон/Аватар</b>", s.buildMediaSettingsMenu())
	case cmsCbAdminHomeAbout:
		return true, s.renderMenu(c, true, "✍️ <b>Главная / О себе</b>", s.buildHomeAboutMenu())
	case cmsCbAdminProjects:
		return true, s.renderMenu(c, true, "🧩 <b>Проекты</b>", s.buildProjectsMenu())
	case cmsCbAdminEvents:
		return true, s.renderMenu(c, true, "📅 <b>Мероприятия</b>", s.buildEventsMenu())
	case cmsCbAdminContacts:
		return true, s.renderMenu(c, true, "📞 <b>Контакты</b>", s.buildContactsMenu())
	case cmsCbAdminBack:
		s.setState(userID, cmsStateIdle)
		return true, tryEdit(c, "Админка. Выберите раздел:", buildAdminMenu(), tele.ModeHTML)
	case cmsCbSetBackground:
		s.setState(userID, cmsStateSetBackgroundMedia)
		return true, tryEdit(c, "Пришлите фото (jpg/png) для фона сайта.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetAvatar:
		s.setState(userID, cmsStateSetAvatarMedia)
		return true, tryEdit(c, "Пришлите фото (jpg/png) для аватара.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetHomeDesc:
		s.setState(userID, cmsStateSetHomeDesc)
		return true, tryEdit(c, "Введите новый текст для блока «Главная».", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetAboutText:
		s.setState(userID, cmsStateSetAboutText)
		return true, tryEdit(c, "Введите новый текст для блока «О себе».", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetContactEmail:
		s.setState(userID, cmsStateSetContactEmail)
		return true, tryEdit(c, "Введите email для контактов.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetContactPhone:
		s.setState(userID, cmsStateSetContactPhone)
		return true, tryEdit(c, "Введите телефон для контактов.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbSetContactLocation:
		s.setState(userID, cmsStateSetContactLocation)
		return true, tryEdit(c, "Введите адрес/локацию для контактов.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbProjectList:
		return true, s.sendProjectsList(c, true)
	case cmsCbProjectAdd:
		s.resetDraft(userID)
		s.setState(userID, cmsStateProjectCreateTitle)
		return true, tryEdit(c, "Создание проекта (1/4): введите название.", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbProjectEdit:
		return true, s.sendProjectPicker(c, false)
	case cmsCbProjectDeleteMenu:
		return true, s.sendProjectPicker(c, true)
	case cmsCbEventList:
		return true, s.sendEventsList(c, true)
	case cmsCbEventAdd:
		s.resetDraft(userID)
		s.setState(userID, cmsStateEventCreatePayload)
		return true, tryEdit(c, "Формат:\n<title> | <date> | <time> | <location> | <max_participants> | <description>\nПример: Встреча | 2026-03-15 | 18:30 | СПб, Невский 1 | 30 | Описание", s.buildBackToCMSMenu(), tele.ModeHTML)
	case cmsCbEventEdit:
		return true, s.sendEventPicker(c, false)
	case cmsCbEventDeleteMenu:
		return true, s.sendEventPicker(c, true)
	}

	if strings.HasPrefix(data, cmsCbProjectPickPrefix) {
		projectID := strings.TrimSpace(strings.TrimPrefix(data, cmsCbProjectPickPrefix))
		if projectID == "" {
			return true, tryEdit(c, "Не удалось определить проект.", s.buildProjectsMenu(), tele.ModeHTML)
		}
		d := s.getDraft(userID)
		d.ProjectID = projectID
		s.setState(userID, cmsStateProjectEditValue)
		return true, tryEdit(c, "Выберите поле для редактирования:", s.buildProjectFieldMenu(), tele.ModeHTML)
	}
	if strings.HasPrefix(data, cmsCbProjectFieldPrefix) {
		field := strings.TrimSpace(strings.TrimPrefix(data, cmsCbProjectFieldPrefix))
		if field == "" {
			return true, tryEdit(c, "Неизвестное поле.", s.buildProjectFieldMenu(), tele.ModeHTML)
		}
		d := s.getDraft(userID)
		if strings.TrimSpace(d.ProjectID) == "" {
			return true, tryEdit(c, "Сначала выберите проект.", s.buildProjectsMenu(), tele.ModeHTML)
		}
		d.Field = field
		s.setState(userID, cmsStateProjectEditValue)
		return true, tryEdit(c, "Введите новое значение поля.", s.buildBackToCMSMenu(), tele.ModeHTML)
	}
	if strings.HasPrefix(data, cmsCbProjectDeletePrefix) {
		projectID := strings.TrimSpace(strings.TrimPrefix(data, cmsCbProjectDeletePrefix))
		if projectID == "" {
			return true, tryEdit(c, "Не удалось определить проект.", s.buildProjectsMenu(), tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.repo.DeleteProject(ctx, projectID); err != nil {
			if errors.Is(err, ErrCMSNotFound) {
				return true, tryEdit(c, "Проект не найден.", s.buildProjectsMenu(), tele.ModeHTML)
			}
			return true, tryEdit(c, "Ошибка удаления проекта: "+err.Error(), s.buildProjectsMenu(), tele.ModeHTML)
		}
		return true, tryEdit(c, "Проект удален.", s.buildProjectsMenu(), tele.ModeHTML)
	}
	if strings.HasPrefix(data, cmsCbEventPickPrefix) {
		eventID := strings.TrimSpace(strings.TrimPrefix(data, cmsCbEventPickPrefix))
		if eventID == "" {
			return true, tryEdit(c, "Не удалось определить мероприятие.", s.buildEventsMenu(), tele.ModeHTML)
		}
		d := s.getDraft(userID)
		d.EventID = eventID
		s.setState(userID, cmsStateEventEditValue)
		return true, tryEdit(c, "Выберите поле для редактирования:", s.buildEventFieldMenu(), tele.ModeHTML)
	}
	if strings.HasPrefix(data, cmsCbEventFieldPrefix) {
		field := strings.TrimSpace(strings.TrimPrefix(data, cmsCbEventFieldPrefix))
		if field == "" {
			return true, tryEdit(c, "Неизвестное поле.", s.buildEventFieldMenu(), tele.ModeHTML)
		}
		d := s.getDraft(userID)
		if strings.TrimSpace(d.EventID) == "" {
			return true, tryEdit(c, "Сначала выберите мероприятие.", s.buildEventsMenu(), tele.ModeHTML)
		}
		d.EventField = field
		s.setState(userID, cmsStateEventEditValue)
		return true, tryEdit(c, "Введите новое значение поля.", s.buildBackToCMSMenu(), tele.ModeHTML)
	}
	if strings.HasPrefix(data, cmsCbEventDeletePrefix) {
		eventID := strings.TrimSpace(strings.TrimPrefix(data, cmsCbEventDeletePrefix))
		if eventID == "" {
			return true, tryEdit(c, "Не удалось определить мероприятие.", s.buildEventsMenu(), tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.repo.DeleteEvent(ctx, eventID); err != nil {
			if errors.Is(err, ErrCMSNotFound) {
				return true, tryEdit(c, "Мероприятие не найдено.", s.buildEventsMenu(), tele.ModeHTML)
			}
			return true, tryEdit(c, "Ошибка удаления мероприятия: "+err.Error(), s.buildEventsMenu(), tele.ModeHTML)
		}
		return true, tryEdit(c, "Мероприятие удалено.", s.buildEventsMenu(), tele.ModeHTML)
	}

	return true, nil
}

func (s *CMSService) HandleBotCMSAdminText(c tele.Context) (bool, error) {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return false, nil
	}
	state := s.getState(c.Sender().ID)
	if state == cmsStateIdle {
		return false, nil
	}
	if s.repo == nil {
		return true, c.Reply("CMS-репозиторий не инициализирован.")
	}
	text := strings.TrimSpace(c.Text())
	if text == "" {
		return true, c.Reply("Пустое значение.")
	}

	switch state {
	case cmsStateSetHomeDesc:
		return true, s.updateSiteSettingsText(c, func(ss *SiteSettings) {
			ss.HomeDescription = text
		}, "Описание главной страницы обновлено.")
	case cmsStateSetAboutText:
		return true, s.updateSiteSettingsText(c, func(ss *SiteSettings) {
			ss.AboutText = text
		}, "Текст «О себе» обновлен.")
	case cmsStateSetContactEmail:
		return true, s.updateSiteSettingsText(c, func(ss *SiteSettings) {
			ss.ContactEmail = text
		}, "Email обновлен.")
	case cmsStateSetContactPhone:
		return true, s.updateSiteSettingsText(c, func(ss *SiteSettings) {
			ss.ContactPhone = text
		}, "Телефон обновлен.")
	case cmsStateSetContactLocation:
		return true, s.updateSiteSettingsText(c, func(ss *SiteSettings) {
			ss.ContactLocation = text
		}, "Локация обновлена.")
	case cmsStateProjectCreateTitle:
		d := s.getDraft(c.Sender().ID)
		d.Project = Project{Title: text}
		s.setState(c.Sender().ID, cmsStateProjectCreateShort)
		return true, c.Reply("Создание проекта (2/4): введите короткое описание.")
	case cmsStateProjectCreateShort:
		d := s.getDraft(c.Sender().ID)
		d.Project.ShortDescription = text
		s.setState(c.Sender().ID, cmsStateProjectCreateDetailed)
		return true, c.Reply("Создание проекта (3/4): введите детальное содержание.")
	case cmsStateProjectCreateDetailed:
		d := s.getDraft(c.Sender().ID)
		d.Project.DetailedContent = text
		s.setState(c.Sender().ID, cmsStateProjectCreateMedia)
		return true, c.Reply("Создание проекта (4/4): введите MediaURL (или '-' если без медиа).")
	case cmsStateProjectCreateMedia:
		d := s.getDraft(c.Sender().ID)
		if text != "-" {
			d.Project.MediaURL = text
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.repo.CreateProject(ctx, &d.Project); err != nil {
			return true, c.Reply("Ошибка создания проекта: " + err.Error())
		}
		s.setState(c.Sender().ID, cmsStateIdle)
		s.resetDraft(c.Sender().ID)
		return true, c.Reply("Проект добавлен.")
	case cmsStateProjectEditValue:
		d := s.getDraft(c.Sender().ID)
		if strings.TrimSpace(d.ProjectID) == "" || strings.TrimSpace(d.Field) == "" {
			return true, c.Reply("Сначала выберите проект и поле через меню CMS.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		project, err := s.repo.GetProjectByID(ctx, d.ProjectID)
		if err != nil {
			if errors.Is(err, ErrCMSNotFound) {
				return true, c.Reply("Проект не найден.")
			}
			return true, c.Reply("Ошибка загрузки проекта: " + err.Error())
		}
		switch d.Field {
		case "title":
			project.Title = text
		case "short":
			project.ShortDescription = text
		case "details":
			project.DetailedContent = text
		case "media":
			project.MediaURL = text
		default:
			return true, c.Reply("Неизвестное поле.")
		}
		if err := s.repo.UpdateProject(ctx, project); err != nil {
			return true, c.Reply("Ошибка обновления проекта: " + err.Error())
		}
		d.Field = ""
		s.setState(c.Sender().ID, cmsStateIdle)
		return true, c.Reply("Проект обновлен.")
	case cmsStateEventCreatePayload:
		title, date, timeStr, location, maxParticipants, description, err := parseBotEventPayload(&tele.Message{Payload: text})
		if err != nil {
			return true, c.Reply("Неверный формат. Пример:\nВстреча | 2026-03-15 | 18:30 | СПб, Невский 1 | 30 | Описание")
		}
		event := &Event{
			Title:               title,
			Description:         description,
			Date:                date,
			Time:                timeStr,
			Location:            location,
			MaxParticipants:     maxParticipants,
			CurrentParticipants: make([]int64, 0),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.repo.CreateEvent(ctx, event); err != nil {
			return true, c.Reply("Ошибка создания мероприятия: " + err.Error())
		}
		s.setState(c.Sender().ID, cmsStateIdle)
		s.resetDraft(c.Sender().ID)
		return true, c.Reply("Мероприятие добавлено.")
	case cmsStateEventEditValue:
		d := s.getDraft(c.Sender().ID)
		if strings.TrimSpace(d.EventID) == "" || strings.TrimSpace(d.EventField) == "" {
			return true, c.Reply("Сначала выберите мероприятие и поле через меню CMS.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		event, err := s.repo.GetEventByID(ctx, d.EventID)
		if err != nil {
			if errors.Is(err, ErrCMSNotFound) {
				return true, c.Reply("Мероприятие не найдено.")
			}
			return true, c.Reply("Ошибка загрузки мероприятия: " + err.Error())
		}
		switch d.EventField {
		case "title":
			event.Title = text
		case "description":
			event.Description = text
		case "date":
			dt, parseErr := parseEventDate(text)
			if parseErr != nil {
				return true, c.Reply("Неверный формат даты. Используйте YYYY-MM-DD или DD.MM.YYYY")
			}
			event.Date = dt
		case "time":
			if _, parseErr := time.Parse("15:04", text); parseErr != nil {
				return true, c.Reply("Неверный формат времени. Используйте HH:MM")
			}
			event.Time = text
		case "location":
			event.Location = text
		case "max":
			maxValue, parseErr := strconv.Atoi(text)
			if parseErr != nil || maxValue < 0 {
				return true, c.Reply("max_participants должен быть целым числом >= 0")
			}
			event.MaxParticipants = maxValue
		default:
			return true, c.Reply("Неизвестное поле.")
		}
		if err := s.repo.UpdateEvent(ctx, event); err != nil {
			return true, c.Reply("Ошибка обновления мероприятия: " + err.Error())
		}
		d.EventField = ""
		s.setState(c.Sender().ID, cmsStateIdle)
		return true, c.Reply("Мероприятие обновлено.")
	}

	return false, nil
}

func (s *CMSService) HandleBotCMSAdminMedia(c tele.Context) (bool, error) {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return false, nil
	}
	state := s.getState(c.Sender().ID)
	if state != cmsStateSetBackgroundMedia && state != cmsStateSetAvatarMedia {
		return false, nil
	}
	if c.Message() == nil {
		return true, c.Reply("Пустое сообщение.")
	}

	mediaPath, err := s.saveTelegramMedia(c.Bot(), c.Message())
	if err != nil {
		return true, c.Reply("Ошибка сохранения файла: " + err.Error())
	}
	if mediaPath == "" {
		return true, c.Reply("Пришлите файл в формате jpg/png.")
	}
	ext := strings.ToLower(filepath.Ext(mediaPath))
	if ext != ".jpg" && ext != ".png" {
		s.removeLocalMedia(mediaPath)
		return true, c.Reply("Для фона и аватара разрешены только jpg/png.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	settings, err := s.ensureSiteSettings(ctx)
	if err != nil {
		return true, c.Reply("Ошибка чтения настроек: " + err.Error())
	}
	if state == cmsStateSetBackgroundMedia {
		settings.BackgroundURL = mediaPath
	} else {
		settings.AvatarURL = mediaPath
	}
	if err := s.repo.UpdateSiteSettings(ctx, settings); err != nil {
		return true, c.Reply("Ошибка обновления настроек: " + err.Error())
	}

	s.setState(c.Sender().ID, cmsStateIdle)
	return true, c.Reply("Файл сохранен и настройки обновлены.")
}

func (s *CMSService) updateSiteSettingsText(c tele.Context, update func(*SiteSettings), okMessage string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	settings, err := s.ensureSiteSettings(ctx)
	if err != nil {
		return c.Reply("Ошибка чтения настроек: " + err.Error())
	}
	update(settings)
	if err := s.repo.UpdateSiteSettings(ctx, settings); err != nil {
		return c.Reply("Ошибка сохранения настроек: " + err.Error())
	}
	s.setState(c.Sender().ID, cmsStateIdle)
	return c.Reply(okMessage)
}

func (s *CMSService) ensureSiteSettings(ctx context.Context) (*SiteSettings, error) {
	settings, err := s.repo.GetSiteSettings(ctx)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, ErrCMSNotFound) {
		return nil, err
	}
	defaults := &SiteSettings{}
	ensureSiteSettingsDefaults(defaults)
	if createErr := s.repo.CreateSiteSettings(ctx, defaults); createErr != nil {
		return nil, createErr
	}
	return defaults, nil
}

func (s *CMSService) sendProjectsList(c tele.Context, edit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	projects, err := s.repo.ListProjects(ctx)
	if err != nil {
		return tryEdit(c, "Ошибка загрузки проектов: "+err.Error(), s.buildProjectsMenu(), tele.ModeHTML)
	}
	if len(projects) == 0 {
		return s.renderMenu(c, edit, "Проекты пока не добавлены.", s.buildProjectsMenu())
	}
	var sb strings.Builder
	sb.WriteString("🧩 <b>Проекты</b>\n\n")
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf("• <code>%s</code> — %s\n", p.ID, html.EscapeString(p.Title)))
	}
	return s.renderMenu(c, edit, sb.String(), s.buildProjectsMenu())
}

func (s *CMSService) sendEventsList(c tele.Context, edit bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := s.repo.ListEvents(ctx)
	if err != nil {
		return tryEdit(c, "Ошибка загрузки мероприятий: "+err.Error(), s.buildEventsMenu(), tele.ModeHTML)
	}
	if len(events) == 0 {
		return s.renderMenu(c, edit, "Мероприятий пока нет.", s.buildEventsMenu())
	}
	var sb strings.Builder
	sb.WriteString("📅 <b>Мероприятия</b>\n\n")
	for _, e := range events {
		sb.WriteString(fmt.Sprintf("• <code>%s</code> — %s (%s %s)\n", e.ID, html.EscapeString(e.Title), e.Date.Format("02.01.2006"), html.EscapeString(e.Time)))
	}
	return s.renderMenu(c, edit, sb.String(), s.buildEventsMenu())
}

func (s *CMSService) sendProjectPicker(c tele.Context, forDelete bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	projects, err := s.repo.ListProjects(ctx)
	if err != nil {
		return tryEdit(c, "Ошибка загрузки проектов: "+err.Error(), s.buildProjectsMenu(), tele.ModeHTML)
	}
	if len(projects) == 0 {
		return tryEdit(c, "Список проектов пуст.", s.buildProjectsMenu(), tele.ModeHTML)
	}

	menu := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(projects)+1)
	for _, p := range projects {
		callback := cmsCbProjectPickPrefix + p.ID
		if forDelete {
			callback = cmsCbProjectDeletePrefix + p.ID
		}
		rows = append(rows, menu.Row(menu.Data(p.Title, callback)))
	}
	rows = append(rows, menu.Row(menu.Data("🔙 Назад", cmsCbAdminProjects)))
	menu.Inline(rows...)

	label := "Выберите проект для редактирования:"
	if forDelete {
		label = "Выберите проект для удаления:"
	}
	return tryEdit(c, label, menu, tele.ModeHTML)
}

func (s *CMSService) sendEventPicker(c tele.Context, forDelete bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := s.repo.ListEvents(ctx)
	if err != nil {
		return tryEdit(c, "Ошибка загрузки мероприятий: "+err.Error(), s.buildEventsMenu(), tele.ModeHTML)
	}
	if len(events) == 0 {
		return tryEdit(c, "Список мероприятий пуст.", s.buildEventsMenu(), tele.ModeHTML)
	}

	menu := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(events)+1)
	for _, e := range events {
		title := fmt.Sprintf("%s (%s)", e.Title, e.Date.Format("02.01"))
		callback := cmsCbEventPickPrefix + e.ID
		if forDelete {
			callback = cmsCbEventDeletePrefix + e.ID
		}
		rows = append(rows, menu.Row(menu.Data(title, callback)))
	}
	rows = append(rows, menu.Row(menu.Data("🔙 Назад", cmsCbAdminEvents)))
	menu.Inline(rows...)

	label := "Выберите мероприятие для редактирования:"
	if forDelete {
		label = "Выберите мероприятие для удаления:"
	}
	return tryEdit(c, label, menu, tele.ModeHTML)
}

func (s *CMSService) renderMenu(c tele.Context, edit bool, text string, menu *tele.ReplyMarkup) error {
	if edit && c.Callback() != nil {
		return tryEdit(c, text, menu, tele.ModeHTML)
	}
	return c.Send(text, menu, tele.ModeHTML)
}

func (s *CMSService) buildSiteAdminMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Фон / Аватар", cmsCbAdminMedia)),
		menu.Row(menu.Data("Главная / О себе", cmsCbAdminHomeAbout)),
		menu.Row(menu.Data("Проекты", cmsCbAdminProjects)),
		menu.Row(menu.Data("Мероприятия", cmsCbAdminEvents)),
		menu.Row(menu.Data("Контакты", cmsCbAdminContacts)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminBack)),
	)
	return menu
}

func (s *CMSService) buildMediaSettingsMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Обновить фон", cmsCbSetBackground)),
		menu.Row(menu.Data("Обновить аватар", cmsCbSetAvatar)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)),
	)
	return menu
}

func (s *CMSService) buildHomeAboutMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("HomeDescription", cmsCbSetHomeDesc)),
		menu.Row(menu.Data("AboutText", cmsCbSetAboutText)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)),
	)
	return menu
}

func (s *CMSService) buildProjectsMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Список", cmsCbProjectList)),
		menu.Row(menu.Data("Добавить", cmsCbProjectAdd)),
		menu.Row(menu.Data("Редактировать", cmsCbProjectEdit)),
		menu.Row(menu.Data("Удалить", cmsCbProjectDeleteMenu)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)),
	)
	return menu
}

func (s *CMSService) buildProjectFieldMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Title", cmsCbProjectFieldPrefix+"title")),
		menu.Row(menu.Data("ShortDescription", cmsCbProjectFieldPrefix+"short")),
		menu.Row(menu.Data("DetailedContent", cmsCbProjectFieldPrefix+"details")),
		menu.Row(menu.Data("MediaURL", cmsCbProjectFieldPrefix+"media")),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminProjects)),
	)
	return menu
}

func (s *CMSService) buildEventsMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Список", cmsCbEventList)),
		menu.Row(menu.Data("Добавить", cmsCbEventAdd)),
		menu.Row(menu.Data("Редактировать", cmsCbEventEdit)),
		menu.Row(menu.Data("Удалить", cmsCbEventDeleteMenu)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)),
	)
	return menu
}

func (s *CMSService) buildEventFieldMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Title", cmsCbEventFieldPrefix+"title")),
		menu.Row(menu.Data("Description", cmsCbEventFieldPrefix+"description")),
		menu.Row(menu.Data("Date", cmsCbEventFieldPrefix+"date")),
		menu.Row(menu.Data("Time", cmsCbEventFieldPrefix+"time")),
		menu.Row(menu.Data("Location", cmsCbEventFieldPrefix+"location")),
		menu.Row(menu.Data("MaxParticipants", cmsCbEventFieldPrefix+"max")),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminEvents)),
	)
	return menu
}

func (s *CMSService) buildContactsMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("Email", cmsCbSetContactEmail)),
		menu.Row(menu.Data("Phone", cmsCbSetContactPhone)),
		menu.Row(menu.Data("Location", cmsCbSetContactLocation)),
		menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)),
	)
	return menu
}

func (s *CMSService) buildBackToCMSMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(menu.Data("🔙 Назад", cmsCbAdminMain)))
	return menu
}

func (s *CMSService) setState(userID int64, state string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.states[userID] = state
}

func (s *CMSService) getState(userID int64) string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.states[userID]
}

func (s *CMSService) resetDraft(userID int64) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	delete(s.drafts, userID)
}

func (s *CMSService) getDraft(userID int64) *cmsBotDraft {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	d, ok := s.drafts[userID]
	if !ok {
		d = &cmsBotDraft{}
		s.drafts[userID] = d
	}
	return d
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

func (s *CMSService) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}

	settings, err := s.repo.GetSiteSettings(r.Context())
	if err != nil {
		if errors.Is(err, ErrCMSNotFound) {
			defaults := &SiteSettings{}
			ensureSiteSettingsDefaults(defaults)
			if createErr := s.repo.CreateSiteSettings(r.Context(), defaults); createErr != nil {
				writeCMSError(w, http.StatusInternalServerError, createErr.Error())
				return
			}
			settings = defaults
		} else {
			writeCMSError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeCMSJSON(w, http.StatusOK, settings)
}

func (s *CMSService) GetProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.repo == nil {
		writeCMSError(w, http.StatusInternalServerError, "repository is not initialized")
		return
	}

	projects, err := s.repo.ListProjects(r.Context())
	if err != nil {
		writeCMSError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusOK, projects)
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
	items, err := s.repo.ListEvents(r.Context())
	if err != nil {
		writeCMSError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusOK, items)
}

func (s *CMSService) GetNews(w http.ResponseWriter, r *http.Request) {
	items, err := s.GetChannelPosts(r.Context())
	if err != nil {
		writeCMSError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeCMSJSON(w, http.StatusOK, items)
}

// GetChannelPosts parses latest Telegram channel posts from public HTML mirror.
func (s *CMSService) GetChannelPosts(ctx context.Context) ([]ChannelPost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, telegramNewsSourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OpheliaCMS/1.0)")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch telegram channel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram channel status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read telegram html: %w", err)
	}
	htmlBody := string(body)

	matches := reChannelMessage.FindAllStringSubmatch(htmlBody, -1)
	posts := make([]ChannelPost, 0, 10)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		if len(posts) >= 10 {
			break
		}

		postRef := strings.TrimSpace(match[1])
		fragment := match[2]
		if postRef == "" {
			continue
		}

		text := extractChannelText(fragment)
		imageURL := extractChannelImageURL(fragment)
		if text == "" && imageURL == "" {
			continue
		}

		posts = append(posts, ChannelPost{
			ID:       strings.ReplaceAll(postRef, "/", "_"),
			Text:     text,
			ImageURL: imageURL,
			PostURL:  "https://t.me/" + postRef,
		})
	}

	return posts, nil
}

func extractChannelText(fragment string) string {
	match := reChannelText.FindStringSubmatch(fragment)
	if len(match) < 2 {
		return ""
	}
	text := match[1]
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = reHTMLTags.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = strings.TrimSpace(text)
	return text
}

func extractChannelImageURL(fragment string) string {
	if m := reChannelBGImage.FindStringSubmatch(fragment); len(m) >= 2 {
		return strings.TrimSpace(html.UnescapeString(m[1]))
	}
	if m := reChannelImage.FindStringSubmatch(fragment); len(m) >= 2 {
		return strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return ""
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
			sb.WriteString(fmt.Sprintf("• %s | %s %s | %s | %d/%d\n",
				event.ID,
				event.Date.Format("02.01.2006"),
				event.Time,
				event.Location,
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
	sb.WriteString(fmt.Sprintf("Дата: %s\n", event.Date.Format("02.01.2006")))
	sb.WriteString(fmt.Sprintf("Время: %s\n", event.Time))
	sb.WriteString(fmt.Sprintf("Локация: %s\n", event.Location))
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

// HandleBotEventAdd creates a CMS event from admin command.
// Format: /cms_event_add <title> | <date> | <time> | <location> | <max_participants> | <description>
func (s *CMSService) HandleBotEventAdd(c tele.Context) error {
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

	title, date, timeStr, location, maxParticipants, description, err := parseBotEventPayload(msg)
	if err != nil {
		return c.Reply("Формат: /cms_event_add <title> | <date> | <time> | <location> | <max_participants> | <description>\nДата: 2006-01-02 или 02.01.2006")
	}

	event := &Event{
		Title:               title,
		Description:         description,
		Date:                date,
		Time:                timeStr,
		Location:            location,
		MaxParticipants:     maxParticipants,
		CurrentParticipants: make([]int64, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return c.Reply("Не удалось создать событие: " + err.Error())
	}
	return c.Reply(fmt.Sprintf("Событие создано. ID: %s", event.ID))
}

// HandleBotPostDelete deletes CMS post by ID (admin only).
// Format: /cms_post_del <post_id>
func (s *CMSService) HandleBotPostDelete(c tele.Context) error {
	if c.Sender() == nil || !isAdmin(c.Sender().ID) {
		return c.Reply("Недостаточно прав.")
	}
	if s.repo == nil {
		return c.Reply("CMS-репозиторий не инициализирован.")
	}
	args := c.Args()
	if len(args) < 1 {
		return c.Reply("Используйте: /cms_post_del <post_id>")
	}
	postID := strings.TrimSpace(args[0])
	if postID == "" {
		return c.Reply("Укажите ID поста.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	post, _ := s.repo.GetPostByID(ctx, postID)
	if err := s.repo.DeletePost(ctx, postID); err != nil {
		if errors.Is(err, ErrCMSNotFound) {
			return c.Reply("Пост не найден.")
		}
		return c.Reply("Не удалось удалить пост: " + err.Error())
	}
	if post != nil {
		s.removeLocalMedia(post.MediaPath)
	}
	return c.Reply("Пост удален.")
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

func parseBotEventPayload(msg *tele.Message) (string, time.Time, string, string, int, string, error) {
	if msg == nil {
		return "", time.Time{}, "", "", 0, "", errors.New("empty message")
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
		return "", time.Time{}, "", "", 0, "", errors.New("empty payload")
	}

	parts := strings.SplitN(raw, "|", 6)
	if len(parts) < 5 {
		return "", time.Time{}, "", "", 0, "", errors.New("invalid payload")
	}

	title := strings.TrimSpace(parts[0])
	dateRaw := strings.TrimSpace(parts[1])
	timeRaw := strings.TrimSpace(parts[2])
	location := strings.TrimSpace(parts[3])
	maxRaw := strings.TrimSpace(parts[4])
	description := ""
	if len(parts) == 6 {
		description = strings.TrimSpace(parts[5])
	}
	if title == "" || dateRaw == "" || timeRaw == "" || location == "" || maxRaw == "" {
		return "", time.Time{}, "", "", 0, "", errors.New("title/date/time/location/max are required")
	}

	date, err := parseEventDate(dateRaw)
	if err != nil {
		return "", time.Time{}, "", "", 0, "", err
	}
	if _, err := time.Parse("15:04", timeRaw); err != nil {
		return "", time.Time{}, "", "", 0, "", errors.New("invalid time format (HH:MM)")
	}
	maxParticipants, err := strconv.Atoi(maxRaw)
	if err != nil || maxParticipants < 0 {
		return "", time.Time{}, "", "", 0, "", errors.New("invalid max_participants")
	}

	return title, date, timeRaw, location, maxParticipants, description, nil
}

func parseEventDate(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02",
		"02.01.2006 15:04",
		"02.01.2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			if layout == "2006-01-02" || layout == "02.01.2006" {
				return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local), nil
			}
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid event date")
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

func (s *CMSService) removeLocalMedia(path string) {
	p := strings.TrimSpace(path)
	if p == "" {
		return
	}
	clean := filepath.Clean(p)
	baseUpload := filepath.Clean(s.uploadDir)
	if clean != baseUpload && !strings.HasPrefix(clean, baseUpload+string(os.PathSeparator)) {
		return
	}
	_ = os.Remove(clean)
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
