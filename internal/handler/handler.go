package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/session"
)

type Handler struct {
	svc      *service.Service
	sessions *session.Manager
	tmpls    map[string]*template.Template
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
	"typeLabel": func(t string) string {
		if t == "income" {
			return "Доход"
		}
		return "Расход"
	},
	"typeClass": func(t string) string {
		if t == "income" {
			return "income"
		}
		return "expense"
	},
	"add": func(a, b int) int { return a + b },
}

func New(svc *service.Service, sm *session.Manager, webFS fs.FS) (*Handler, error) {
	h := &Handler{svc: svc, sessions: sm}
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

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /login", h.getLogin)
	mux.HandleFunc("POST /login", h.postLogin)
	mux.HandleFunc("GET /register", h.getRegister)
	mux.HandleFunc("POST /register", h.postRegister)
	mux.HandleFunc("POST /logout", h.postLogout)

	// Protected
	mux.Handle("GET /{$}", h.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})))
	mux.Handle("GET /dashboard", h.auth(http.HandlerFunc(h.getDashboard)))

	mux.Handle("GET /wallets", h.auth(http.HandlerFunc(h.getWallets)))
	mux.Handle("POST /wallets", h.auth(http.HandlerFunc(h.postWallet)))
	mux.Handle("GET /wallets/{id}", h.auth(http.HandlerFunc(h.getWallet)))
	mux.Handle("GET /wallets/{id}/edit", h.auth(http.HandlerFunc(h.getWalletEdit)))
	mux.Handle("PUT /wallets/{id}", h.auth(http.HandlerFunc(h.putWallet)))
	mux.Handle("DELETE /wallets/{id}", h.auth(http.HandlerFunc(h.deleteWallet)))

	mux.Handle("GET /categories", h.auth(http.HandlerFunc(h.getCategories)))
	mux.Handle("POST /categories", h.auth(http.HandlerFunc(h.postCategory)))
	mux.Handle("GET /categories/{id}", h.auth(http.HandlerFunc(h.getCategory)))
	mux.Handle("GET /categories/{id}/edit", h.auth(http.HandlerFunc(h.getCategoryEdit)))
	mux.Handle("PUT /categories/{id}", h.auth(http.HandlerFunc(h.putCategory)))
	mux.Handle("DELETE /categories/{id}", h.auth(http.HandlerFunc(h.deleteCategory)))
	mux.Handle("GET /categories/options", h.auth(http.HandlerFunc(h.getCategoryOptions)))

	mux.Handle("GET /operations", h.auth(http.HandlerFunc(h.getOperations)))
	mux.Handle("POST /operations", h.auth(http.HandlerFunc(h.postOperation)))
	mux.Handle("GET /operations/{id}", h.auth(http.HandlerFunc(h.getOperationRow)))
	mux.Handle("GET /operations/{id}/edit", h.auth(http.HandlerFunc(h.getOperationEdit)))
	mux.Handle("PUT /operations/{id}", h.auth(http.HandlerFunc(h.putOperation)))
	mux.Handle("DELETE /operations/{id}", h.auth(http.HandlerFunc(h.deleteOperation)))

	return mux
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, page, name string, data any) {
	if err := h.tmpls[page].ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
