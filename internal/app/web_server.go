package app

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultWebAddr         = ":8080"
	defaultFrontendDevAddr = "http://127.0.0.1:4200"
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
	mux.HandleFunc("/cms/settings", cmsService.GetSettings)
	mux.HandleFunc("/cms/projects", cmsService.GetProjects)
	mux.HandleFunc("/cms/news", cmsService.GetNews)
	mux.Handle("/cms/events/register", requireValidUserID(http.HandlerFunc(cmsService.RegisterForEvent)))

	uploadsFS := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cmsUploadsDir)))
	mux.Handle("/uploads/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeCMSError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		uploadsFS.ServeHTTP(w, r)
	}))

	frontendRoot := resolveFrontendBuildRoot()
	frontendDevURL := strings.TrimSpace(os.Getenv("OPHELIA_FRONTEND_DEV_URL"))
	if frontendDevURL == "" {
		frontendDevURL = defaultFrontendDevAddr
	}
	log.Printf("📦 Frontend root: %s", frontendRoot)

	handler := spaFallbackHandler(mux, frontendRoot, frontendDevURL)
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

func resolveFrontendBuildRoot() string {
	if custom := strings.TrimSpace(os.Getenv("OPHELIA_FRONTEND_DIST")); custom != "" {
		if isDir(custom) {
			return custom
		}
		log.Printf("⚠️ OPHELIA_FRONTEND_DIST does not exist or is not a directory: %s", custom)
	}

	relCandidates := []string{
		"frontend/dist/ophelia/browser",
		"frontend/dist/ophelia",
		"frontend/dist/browser",
		"frontend/dist",
	}

	bases := []string{"."}
	if _, file, _, ok := runtime.Caller(0); ok {
		projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
		bases = append(bases, projectRoot)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		bases = append(bases, exeDir, filepath.Dir(exeDir), filepath.Dir(filepath.Dir(exeDir)))
	}

	tried := make([]string, 0, len(bases)*len(relCandidates))
	for _, base := range dedupeStrings(bases) {
		for _, rel := range relCandidates {
			candidate := filepath.Clean(filepath.Join(base, rel))
			tried = append(tried, candidate)
			if isDir(candidate) {
				return candidate
			}
		}
	}

	log.Printf("⚠️ Frontend build dir not found. Checked: %s", strings.Join(tried, ", "))
	return filepath.Join(".", relCandidates[0])
}

func requireValidUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			requireCMSJWT(next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireAdminIDForCreatePost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			requireCMSAdminJWT(next).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaFallbackHandler(apiMux *http.ServeMux, frontendRoot, frontendDevURL string) http.Handler {
	indexPath := resolveIndexFile(frontendRoot)
	indexExists := indexPath != ""

	staticFS := http.FileServer(http.Dir(frontendRoot))
	absRoot, err := filepath.Abs(frontendRoot)
	if err != nil {
		absRoot = frontendRoot
	}

	devProxy := newDevProxy(frontendDevURL)
	if !indexExists && devProxy != nil {
		log.Printf("⚠️ Frontend index file not found under %s, proxying to %s", frontendRoot, frontendDevURL)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, pattern := apiMux.Handler(r); pattern != "" {
			h.ServeHTTP(w, r)
			return
		}

		if indexExists && tryServeStaticAsset(staticFS, absRoot, r, w) {
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/cms/") ||
			strings.HasPrefix(r.URL.Path, "/uploads/") {
			http.NotFound(w, r)
			return
		}

		if indexExists {
			http.ServeFile(w, r, indexPath)
			return
		}

		if devProxy != nil {
			devProxy.ServeHTTP(w, r)
			return
		}

		writeCMSError(w, http.StatusServiceUnavailable, "frontend build not found")
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

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func newDevProxy(raw string) http.Handler {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		log.Printf("⚠️ Invalid OPHELIA_FRONTEND_DEV_URL: %s", raw)
		return nil
	}
	return httputil.NewSingleHostReverseProxy(u)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func resolveIndexFile(frontendRoot string) string {
	candidates := []string{
		filepath.Join(frontendRoot, "index.html"),
		filepath.Join(frontendRoot, "index.csr.html"),
	}
	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate
		}
	}
	return ""
}
