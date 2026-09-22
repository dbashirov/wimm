package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/dbashirov/wimm/internal/session"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type contextKey string

const userIDKey contextKey = "userID"

func userIDFromCtx(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey).(uuid.UUID)
	return id
}

func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := h.sessions.Get(r.Context(), r)
		if err != nil {
			if errors.Is(err, session.ErrNoSession) {
				redirectToLogin(w, r)
				return
			}
			slog.Error("auth",
				"method", r.Method, "path", r.URL.Path, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// redirectToLogin уводит на страницу входа. На htmx-запрос 302 не годится:
// htmx молча пройдёт по редиректу и вставит форму логина внутрь цели запроса.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// recoverPanic ловит панику в любом хендлере, логирует stack и возвращает 500.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("PANIC",
					"method", r.Method, "path", r.URL.Path,
					"value", v, "stack", string(debug.Stack()))
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder перехватывает статус ответа для логирования.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.wroteHeader {
		sr.status = code
		sr.wroteHeader = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if !sr.wroteHeader {
		sr.status = http.StatusOK
		sr.wroteHeader = true
	}
	return sr.ResponseWriter.Write(b)
}

// logRequests пишет access log для каждого запроса.
func logRequests(ips *ipResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sr, r)
			slog.Info("http",
				"ip", ips.clientIP(r),
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// rateLimiter — простой token bucket per-key с автоматической очисткой
// старых записей. Безопасен для конкурентного доступа.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newRateLimiter(r rate.Limit, burst int) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) get(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if v, ok := rl.visitors[key]; ok {
		v.lastSeen = time.Now()
		return v.limiter
	}
	l := rate.NewLimiter(rl.rate, rl.burst)
	rl.visitors[key] = &visitor{limiter: l, lastSeen: time.Now()}
	return l
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for k, v := range rl.visitors {
			if time.Since(v.lastSeen) > time.Hour {
				delete(rl.visitors, k)
			}
		}
		rl.mu.Unlock()
	}
}

// limitByIP — middleware с rate-limit по IP клиента.
func limitByIP(rl *rateLimiter, ips *ipResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.get(ips.clientIP(r)).Allow() {
				http.Error(w, "слишком много запросов, попробуйте позже",
					http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders добавляет заголовки безопасности ко всем ответам.
func securityHeaders(hsts bool) func(http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' " +
		"https://cdn.jsdelivr.net https://unpkg.com https://npmcdn.com " +
		"https://cdn.tailwindcss.com; " +
		"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net " +
		"https://fonts.googleapis.com; " +
		"font-src 'self' data: https://fonts.gstatic.com; " +
		"img-src 'self' data:; " +
		"connect-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Content-Security-Policy", csp)
			h.Set("Permissions-Policy",
				"camera=(), microphone=(), geolocation=(), interest-cohort=()")
			if hsts {
				h.Set("Strict-Transport-Security",
					"max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// limitBody ограничивает размер тела запроса для защиты от DoS.
func limitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// requestTimeout ставит общий дедлайн для запроса (наследуется в DB-вызовы).
func requestTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ipResolver определяет IP клиента. Заголовкам X-Forwarded-For/X-Real-IP
// верим, только если соединение пришло от доверенного прокси: иначе клиент
// подставляет их сам и получает свежий rate-limit на каждый запрос.
type ipResolver struct {
	trusted []netip.Prefix
}

// newIPResolver разбирает список доверенных прокси через запятую:
// CIDR ("10.0.0.0/8") или одиночные адреса ("127.0.0.1").
func newIPResolver(proxies string) (*ipResolver, error) {
	var ir ipResolver
	for _, item := range strings.Split(proxies, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			addr, err := netip.ParseAddr(item)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", item, err)
			}
			ir.trusted = append(ir.trusted, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", item, err)
		}
		ir.trusted = append(ir.trusted, prefix.Masked())
	}
	return &ir, nil
}

func (ir *ipResolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range ir.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (ir *ipResolver) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !ir.isTrusted(peer) {
		return host
	}

	// X-Forwarded-For читаем справа: левые элементы может дописать сам клиент,
	// первый недоверенный адрес с конца — тот, кто пришёл к нашему прокси
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		addr, err := netip.ParseAddr(hop)
		if err != nil {
			return host
		}
		if !ir.isTrusted(addr) {
			return addr.Unmap().String()
		}
	}
	if realIP, err := netip.ParseAddr(r.Header.Get("X-Real-IP")); err == nil {
		return realIP.Unmap().String()
	}
	return host
}
