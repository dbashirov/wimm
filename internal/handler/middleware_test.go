package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dbashirov/wimm/web"
)

func TestClientIP(t *testing.T) {
	ips, err := newIPResolver("127.0.0.1, ::1, 10.0.0.0/8")
	if err != nil {
		t.Fatalf("newIPResolver: %v", err)
	}

	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		realIP     string
		want       string
	}{
		{
			name:       "прямое подключение без заголовков",
			remoteAddr: "203.0.113.7:5000",
			want:       "203.0.113.7",
		},
		{
			name:       "недоверенный клиент подделывает X-Forwarded-For",
			remoteAddr: "203.0.113.7:5000",
			xff:        []string{"1.2.3.4"},
			realIP:     "5.6.7.8",
			want:       "203.0.113.7",
		},
		{
			name:       "nginx на localhost",
			remoteAddr: "127.0.0.1:40000",
			xff:        []string{"198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "клиент дописал свой адрес слева — берём ближайший к прокси",
			remoteAddr: "127.0.0.1:40000",
			xff:        []string{"1.2.3.4, 198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "цепочка доверенных прокси пропускается",
			remoteAddr: "127.0.0.1:40000",
			xff:        []string{"198.51.100.1, 10.1.2.3"},
			want:       "198.51.100.1",
		},
		{
			name:       "несколько заголовков X-Forwarded-For",
			remoteAddr: "127.0.0.1:40000",
			xff:        []string{"1.2.3.4", "198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "мусор в X-Forwarded-For — адрес прокси",
			remoteAddr: "127.0.0.1:40000",
			xff:        []string{"not-an-ip"},
			want:       "127.0.0.1",
		},
		{
			name:       "X-Real-IP от доверенного прокси без X-Forwarded-For",
			remoteAddr: "[::1]:40000",
			realIP:     "198.51.100.2",
			want:       "198.51.100.2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}
			if got := ips.clientIP(req); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewIPResolverInvalid(t *testing.T) {
	for _, proxies := range []string{"localhost", "10.0.0.0/33"} {
		if _, err := newIPResolver(proxies); err == nil {
			t.Errorf("newIPResolver(%q): ожидалась ошибка", proxies)
		}
	}
}

func TestRedirectToLogin(t *testing.T) {
	t.Run("обычный запрос — 302", func(t *testing.T) {
		rec := httptest.NewRecorder()
		redirectToLogin(rec, httptest.NewRequest(http.MethodGet, "/wallets", nil))
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
			t.Errorf("got %d Location=%q", rec.Code, rec.Header().Get("Location"))
		}
	})
	t.Run("htmx — HX-Redirect без 302", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		redirectToLogin(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("HX-Redirect"); got != "/login" {
			t.Errorf("HX-Redirect = %q, want /login", got)
		}
	})
}

// TestStaticGetsSecurityHeaders — статика должна идти через общий стек
// middleware, а не мимо него.
func TestStaticGetsSecurityHeaders(t *testing.T) {
	h, err := New(nil, nil, web.FS, Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec,
		httptest.NewRequest(http.MethodGet, "/static/favicon-v2.svg", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, header := range []string{"X-Content-Type-Options", "Content-Security-Policy"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("нет заголовка %s", header)
		}
	}
}

// TestCrossOriginProtection - мутирующие запросы с чужого сайта отклоняются
// до хендлера, включая /login и /register, где CSRF-токена ещё нет.
func TestCrossOriginProtection(t *testing.T) {
	h, err := New(nil, nil, web.FS, Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	routes := h.Routes()

	tests := []struct {
		name    string
		path    string
		host    string
		headers map[string]string
		want    int
	}{
		{
			// nginx терминирует HTTPS и передаёт публичный Host по HTTP
			name:    "старый браузер за nginx: Origin совпадает с Host",
			path:    "/nope",
			host:    "wimm.example",
			headers: map[string]string{"Origin": "https://wimm.example"},
			want:    http.StatusNotFound,
		},
		{
			name:    "Origin: null (sandbox iframe, data:)",
			path:    "/login",
			headers: map[string]string{"Origin": "null"},
			want:    http.StatusForbidden,
		},
		{
			name:    "login с чужого сайта",
			path:    "/login",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			want:    http.StatusForbidden,
		},
		{
			name:    "register с чужого сайта",
			path:    "/register",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"},
			want:    http.StatusForbidden,
		},
		{
			name:    "защищённый маршрут с чужого сайта",
			path:    "/wallets",
			headers: map[string]string{"Sec-Fetch-Site": "same-site"},
			want:    http.StatusForbidden,
		},
		{
			name:    "старый браузер: Origin не совпадает с Host",
			path:    "/login",
			headers: map[string]string{"Origin": "https://evil.example"},
			want:    http.StatusForbidden,
		},
		{
			// несуществующий маршрут: 404 значит, что запрос прошёл проверку
			name:    "тот же origin пропускается",
			path:    "/nope",
			headers: map[string]string{"Sec-Fetch-Site": "same-origin"},
			want:    http.StatusNotFound,
		},
		{
			name: "без заголовков браузера (curl) пропускается",
			path: "/nope",
			want: http.StatusNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.host != "" {
				req.Host = tc.host
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
