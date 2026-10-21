package storage_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dbashirov/wimm/internal/session"
	"github.com/dbashirov/wimm/internal/storage"
)

// Выход через session.Manager поверх живой БД. Тест в storage_test по той же
// причине, что и report_e2e_test.go: PostgreSQL поднят только здесь.

// loginCookie создаёт сессию и возвращает запрос с её cookie.
func loginCookie(t *testing.T, sm *session.Manager, email string) *http.Request {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := sm.Create(context.Background(), rec, createUser(t, email)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

// clearedSessionCookie - cookie session_id в ответе очищена.
func clearedSessionCookie(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session_id" && c.Value == "" && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

func TestSessionManager_Delete(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	s := storage.NewFromPool(testPool)
	sm := session.NewManager(s.Sessions, time.Hour, true)

	req := loginCookie(t, sm, "logout@test.com")
	if _, err := sm.Get(ctx, req); err != nil {
		t.Fatalf("Get до выхода: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := sm.Delete(ctx, rec, req); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !clearedSessionCookie(rec) {
		t.Error("cookie сессии не очищена")
	}
	if _, err := sm.Get(ctx, req); err != session.ErrNoSession {
		t.Errorf("Get после выхода: %v, want ErrNoSession", err)
	}
}

// TestSessionManager_DeleteFailure - при сбое БД cookie всё равно очищается,
// а ошибка возвращается: раньше браузер оставался залогиненным молча.
func TestSessionManager_DeleteFailure(t *testing.T) {
	cleanDB(t)
	s := storage.NewFromPool(testPool)
	sm := session.NewManager(s.Sessions, time.Hour, true)
	req := loginCookie(t, sm, "logout-fail@test.com")

	failed, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	if err := sm.Delete(failed, rec, req); err == nil {
		t.Error("Delete с отменённым контекстом: ожидалась ошибка")
	}
	if !clearedSessionCookie(rec) {
		t.Error("cookie сессии не очищена при сбое удаления")
	}
}
