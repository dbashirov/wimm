package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/session"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// dateLayout — формат <input type="date"> и границ периода в query-строке.
const dateLayout = "2006-01-02"

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
		neg := amount < 0
		if neg {
			amount = -amount
		}
		intPart := int64(amount)
		frac := int64((amount-float64(intPart))*100 + 0.5)
		s := fmt.Sprintf("%d", intPart)
		// вставляем разделители тысяч
		n := len(s)
		var out []byte
		for i, c := range s {
			if i > 0 && (n-i)%3 == 0 {
				out = append(out, ' ')
			}
			out = append(out, byte(c))
		}
		result := fmt.Sprintf("%s.%02d", out, frac)
		if neg {
			return "-" + result
		}
		return result
	},
	// num выводит число для вставки в JS (без разделителей, с точкой)
	"num": func(amount float64) string {
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
	// jsonData сериализует данные для вставки в JS-литерал.
	// template.JS отключает контекстное экранирование, поэтому только как
	// значение внутри <script>: в атрибуте или href это будет XSS.
	"jsonData": func(v any) (template.JS, error) {
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("jsonData: %w", err)
		}
		return template.JS(data), nil
	},
	"hasID": func(ids []uuid.UUID, id uuid.UUID) bool {
		return slices.Contains(ids, id)
	},
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
	pages := []string{
		"login", "register", "dashboard", "wallets",
		"categories", "operations", "wallet_stats", "reports",
	}
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
	mux.Handle("GET /wallets/{id}/stats", h.protected(h.getWalletStats))
	mux.Handle("GET /wallets/{id}/edit", h.protected(h.getWalletEdit))
	mux.Handle("PUT /wallets/{id}", h.protected(h.putWallet))
	mux.Handle("DELETE /wallets/{id}", h.protected(h.deleteWallet))
	mux.Handle("POST /wallets/{id}/toggle-hidden", h.protected(h.postWalletToggleHidden))
	mux.Handle("POST /wallets/{id}/set-default", h.protected(h.postWalletSetDefault))
	mux.Handle("POST /wallets/reorder", h.protected(h.postWalletsReorder))

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

	mux.Handle("GET /reports", h.protected(h.getReport))

	mux.Handle("POST /transfers", h.protected(h.postTransfer))
	mux.Handle("GET /transfers/{transfer_id}", h.protected(h.getTransferRows))
	mux.Handle("GET /transfers/{transfer_id}/edit", h.protected(h.getTransferEdit))
	mux.Handle("PUT /transfers/{transfer_id}", h.protected(h.putTransfer))
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
	slog.Error("handler",
		"op", op, "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func handleDeleteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrHasOperations):
		http.Error(w,
			"Нельзя удалить: есть связанные операции. Сначала удалите операции.",
			http.StatusConflict,
		)
	default:
		slog.Error("handler",
			"op", "delete", "method", r.Method, "path", r.URL.Path, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("render", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, page, name string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, name, data); err != nil {
		slog.Error("renderPartial", "page", page, "name", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
