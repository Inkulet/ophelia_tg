package app

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultWebAddr = ":8080"
	frontendDist   = "frontend/dist/browser"
)

func startWebServer(addr string, cmsService *CMSService) {
	if strings.TrimSpace(addr) == "" {
		addr = defaultWebAddr
	}
	if cmsService == nil {
		log.Printf("⚠️ Web server skipped: CMS service is nil")
		return
	}
	if err := os.MkdirAll(cmsUploadsDir, 0755); err != nil {
		log.Printf("⚠️ Could not create uploads dir: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/cms/posts", requireAdminIDForCreatePost(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cmsService.GetPosts(w, r)
		case http.MethodPost:
			cmsService.CreatePost(w, r)
		default:
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})))
	mux.HandleFunc("/cms/events", cmsService.GetEvents)
	mux.Handle("/cms/events/register", requireValidUserID(http.HandlerFunc(cmsService.RegisterForEvent)))

	uploadsFS := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cmsUploadsDir)))
	mux.Handle("/uploads/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		uploadsFS.ServeHTTP(w, r)
	}))

	handler := spaFallbackHandler(mux, frontendDist)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("✅ Web server started at %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("⚠️ Web server stopped: %v", err)
	}
}

func requireValidUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if _, err := extractCMSUserID(r); err != nil {
				writeCMSError(w, http.StatusUnauthorized, "valid user_id is required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requireAdminIDForCreatePost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			userID, err := extractCMSUserID(r)
			if err != nil {
				writeCMSError(w, http.StatusUnauthorized, "valid admin user_id is required")
				return
			}
			if !isAdmin(userID) {
				writeCMSError(w, http.StatusForbidden, "admin role is required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func spaFallbackHandler(apiMux *http.ServeMux, frontendRoot string) http.Handler {
	indexPath := filepath.Join(frontendRoot, "index.html")
	staticFS := http.FileServer(http.Dir(frontendRoot))
	absRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		absRoot = frontendRoot
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, pattern := apiMux.Handler(r); pattern != "" {
			h.ServeHTTP(w, r)
			return
		}

		if tryServeStaticAsset(staticFS, absRoot, r, w) {
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/cms/") ||
			strings.HasPrefix(r.URL.Path, "/uploads/") {
			http.NotFound(w, r)
			return
		}

		if _, err := os.Stat(indexPath); err != nil {
			writeCMSError(w, http.StatusServiceUnavailable, "frontend build not found")
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func tryServeStaticAsset(fs http.Handler, root string, r *http.Request, w http.ResponseWriter) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	cleanURLPath := path.Clean("/" + r.URL.Path)
	if cleanURLPath == "/" {
		return false
	}
	rel := strings.TrimPrefix(cleanURLPath, "/")
	target := filepath.Join(root, filepath.FromSlash(rel))

	rootClean := filepath.Clean(root)
	targetClean := filepath.Clean(target)
	if targetClean != rootClean && !strings.HasPrefix(targetClean, rootClean+string(os.PathSeparator)) {
		return false
	}

	info, err := os.Stat(targetClean)
	if err != nil || info.IsDir() {
		return false
	}

	fs.ServeHTTP(w, r)
	return true
}
