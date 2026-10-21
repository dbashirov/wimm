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
		h.serverError(w, r, "Login", err)
		return
	}

	if err := h.sessions.Create(r.Context(), w, user.ID); err != nil {
		h.serverError(w, r, "sessions.Create", err)
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
		if errors.Is(err, service.ErrInvalidEmail) {
			h.render(w, "register", registerData{NoNav: true, Error: "Некорректный email"})
			return
		}
		if errors.Is(err, service.ErrEmailTaken) {
			h.render(w, "register", registerData{NoNav: true, Error: "Email уже зарегистрирован"})
			return
		}
		h.serverError(w, r, "Register", err)
		return
	}

	if err := h.sessions.Create(r.Context(), w, user.ID); err != nil {
		h.serverError(w, r, "sessions.Create", err)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *Handler) postLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Delete(r.Context(), w, r); err != nil {
		// cookie уже очищена, но сессия в БД жива: не делаем вид, что выход удался
		h.serverError(w, r, "sessions.Delete", err)
		return
	}
	redirectToLogin(w, r)
}
