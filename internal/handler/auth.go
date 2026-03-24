package handler

import (
	"errors"
	"net/http"

	"github.com/dbashirov/wimm/internal/service"
)

type loginData struct {
	NoNav bool
	Error string
}

type registerData struct {
	NoNav bool
	Error string
}

func (h *Handler) getLogin(w http.ResponseWriter, r *http.Request) {
	h.render(w, "login", loginData{NoNav: true})
}

func (h *Handler) postLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.svc.Login(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			h.render(w, "login", loginData{NoNav: true, Error: "Неверный email или пароль"})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.Create(r.Context(), w, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handler) getRegister(w http.ResponseWriter, r *http.Request) {
	h.render(w, "register", registerData{NoNav: true})
}

func (h *Handler) postRegister(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	if len(password) < 8 {
		h.render(w, "register", registerData{NoNav: true, Error: "Пароль должен быть не менее 8 символов"})
		return
	}

	user, err := h.svc.Register(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			h.render(w, "register", registerData{NoNav: true, Error: "Email уже зарегистрирован"})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessions.Create(r.Context(), w, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handler) postLogout(w http.ResponseWriter, r *http.Request) {
	_ = h.sessions.Delete(r.Context(), w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}
