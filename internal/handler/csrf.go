package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"time"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeader     = "X-CSRF-Token"
	csrfFormField  = "csrf_token"
	sessionCookie  = "session_id"
)

// csrfManager выпускает и проверяет CSRF-токены по схеме double-submit:
// токен = HMAC-SHA256(sessionID, secret), кладётся в cookie (не HttpOnly,
// чтобы JS htmx мог прочитать) и проверяется на мутирующих запросах
// в заголовке X-CSRF-Token либо в форменном поле csrf_token.
type csrfManager struct {
	secret []byte
	secure bool
}

func newCSRFManager(secret string, secure bool) (*csrfManager, error) {
	var s []byte
	if secret != "" {
		s = []byte(secret)
		if len(s) < 16 {
			return nil, errors.New("CSRF_SECRET слишком короткий, минимум 16 байт")
		}
	} else {
		s = make([]byte, 32)
		if _, err := rand.Read(s); err != nil {
			return nil, err
		}
		log.Println("CSRF_SECRET не задан — сгенерирован случайный ключ; пользователи увидят CSRF-ошибку при перезапуске")
	}
	return &csrfManager{secret: s, secure: secure}, nil
}

func (cm *csrfManager) tokenFor(sessionID string) string {
	mac := hmac.New(sha256.New, cm.secret)
	mac.Write([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ensureCookie ставит cookie если её нет или значение устарело.
func (cm *csrfManager) ensureCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
	expected := cm.tokenFor(sessionID)
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value == expected {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    expected,
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   cm.secure,
	})
}

func (cm *csrfManager) validate(r *http.Request, sessionID string) bool {
	expected := cm.tokenFor(sessionID)
	sent := r.Header.Get(csrfHeader)
	if sent == "" {
		sent = r.FormValue(csrfFormField)
	}
	if sent == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(sent))
}

// csrfProtect: для GET/HEAD выпускает cookie, для мутирующих методов —
// валидирует. Должен применяться после auth (чтобы знать sessionID).
func (h *Handler) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		sessionID := cookie.Value

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			h.csrf.ensureCookie(w, r, sessionID)
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !h.csrf.validate(r, sessionID) {
				log.Printf("CSRF rejected %s %s from %s",
					r.Method, r.URL.Path, clientIP(r))
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
