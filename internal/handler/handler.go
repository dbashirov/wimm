package handler

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/session"
	"golang.org/x/time/rate"
)

// Config — параметры конфигурации обработчика, влияющие на безопасность.
type Config struct {
	CSRFSecret    string
	SecureCookies bool // true в проде (HTTPS): cookies с Secure-флагом
	HSTS          bool // true в проде: добавлять Strict-Transport-Security
}

type Handler struct {
	svc          *service.Service
	sessions     *session.Manager
	tmpls        map[string]*template.Template
	loginLimiter *rateLimiter
	csrf         *csrfManager
	cfg          Config
}

// basePage встраивается во все структуры данных страниц,
// чтобы base.html всегда мог обратиться к полю NoNav.
type basePage struct {
	NoNav bool
}

var tmplFuncs = template.FuncMap{
	"money": func(amount float64) string {
		return fmt.Sprintf("%.2f", amount)
	},
	"typeLabel": func(t model.OperationType) string {
		if t == model.TypeIncome {
			return "Доход"
		}
		return "Расход"
	},
	"typeClass": func(t model.OperationType) string {
		if t == model.TypeIncome {
			return string(model.TypeIncome)
		}
		return string(model.TypeExpense)
	},
	"add": func(a, b int) int { return a + b },
}

func New(
	svc *service.Service,
	sm *session.Manager,
	webFS fs.FS,
	cfg Config,
) (*Handler, error) {
	csrf, err := newCSRFManager(cfg.CSRFSecret, cfg.SecureCookies)
	if err != nil {
		return nil, fmt.Errorf("newCSRFManager: %w", err)
	}
	h := &Handler{
		svc:          svc,
		sessions:     sm,
		loginLimiter: newRateLimiter(rate.Every(time.Minute), 5),
		csrf:         csrf,
		cfg:          cfg,
	}
	tmpls, err := parseTemplates(webFS)
	if err != nil {
		return nil, fmt.Errorf("parseTemplates: %w", err)
	}
	h.tmpls = tmpls
	return h, nil
}

func parseTemplates(webFS fs.FS) (map[string]*template.Template, error) {
	pages := []string{"login", "register", "dashboard", "wallets", "categories", "operations"}
	tmpls := make(map[string]*template.Template)
	for _, page := range pages {
		t, err := template.New("").Funcs(tmplFuncs).ParseFS(webFS,
			"templates/base.html",
			"templates/"+page+".html",
			"templates/partials/*.html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		tmpls[page] = t
	}
	return tmpls, nil
}

// protected оборачивает обработчик в auth + csrf.
func (h *Handler) protected(handler http.HandlerFunc) http.Handler {
	return h.auth(h.csrfProtect(handler))
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	loginRL := limitByIP(h.loginLimiter)

	// Healthcheck — без middleware, без auth
	mux.HandleFunc("GET /health", h.getHealth)
	mux.HandleFunc("GET /ready", h.getReady)

	// Public
	mux.HandleFunc("GET /login", h.getLogin)
	mux.Handle("POST /login", loginRL(http.HandlerFunc(h.postLogin)))
	mux.HandleFunc("GET /register", h.getRegister)
	mux.Handle("POST /register", loginRL(http.HandlerFunc(h.postRegister)))
	mux.HandleFunc("POST /logout", h.postLogout)

	// Protected: auth + CSRF
	mux.Handle("GET /{$}", h.protected(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}))
	mux.Handle("GET /dashboard", h.protected(h.getDashboard))

	mux.Handle("GET /wallets", h.protected(h.getWallets))
	mux.Handle("POST /wallets", h.protected(h.postWallet))
	mux.Handle("GET /wallets/{id}", h.protected(h.getWallet))
	mux.Handle("GET /wallets/{id}/edit", h.protected(h.getWalletEdit))
	mux.Handle("PUT /wallets/{id}", h.protected(h.putWallet))
	mux.Handle("DELETE /wallets/{id}", h.protected(h.deleteWallet))
	mux.Handle("POST /wallets/{id}/toggle-hidden", h.protected(h.postWalletToggleHidden))

	mux.Handle("GET /categories", h.protected(h.getCategories))
	mux.Handle("POST /categories", h.protected(h.postCategory))
	mux.Handle("GET /categories/{id}", h.protected(h.getCategory))
	mux.Handle("GET /categories/{id}/edit", h.protected(h.getCategoryEdit))
	mux.Handle("PUT /categories/{id}", h.protected(h.putCategory))
	mux.Handle("DELETE /categories/{id}", h.protected(h.deleteCategory))
	mux.Handle("GET /categories/options", h.protected(h.getCategoryOptions))

	mux.Handle("GET /operations", h.protected(h.getOperations))
	mux.Handle("POST /operations", h.protected(h.postOperation))
	mux.Handle("GET /operations/{id}", h.protected(h.getOperationRow))
	mux.Handle("GET /operations/{id}/edit", h.protected(h.getOperationEdit))
	mux.Handle("PUT /operations/{id}", h.protected(h.putOperation))
	mux.Handle("DELETE /operations/{id}", h.protected(h.deleteOperation))

	mux.Handle("POST /transfers", h.protected(h.postTransfer))
	mux.Handle("DELETE /transfers/{transfer_id}", h.protected(h.deleteTransfer))

	// Глобальный стек middleware: лог → recovery → headers → body limit → timeout → mux
	var handler http.Handler = mux
	handler = requestTimeout(15 * time.Second)(handler)
	handler = limitBody(1 << 20)(handler) // 1 MB
	handler = securityHeaders(h.cfg.HSTS)(handler)
	handler = recoverPanic(handler)
	handler = logRequests(handler)
	return handler
}

// serverError логирует ошибку и возвращает клиенту 500.
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	log.Printf("%s %s %s: %v", r.Method, r.URL.Path, op, err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func handleDeleteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, service.ErrNotFound) {
		http.NotFound(w, r)
	} else {
		log.Printf("%s %s delete: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("render %s: %v", page, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, page, name string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, name, data); err != nil {
		log.Printf("renderPartial %s/%s: %v", page, name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
