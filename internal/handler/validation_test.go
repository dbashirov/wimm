package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/dbashirov/wimm/internal/storage/mocks"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// TestPutWalletErrors - ошибки ввода и чужой кошелёк больше не превращаются в 500.
func TestPutWalletErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	wallets := mocks.NewMockWalletRepo(ctrl)
	svc := service.NewWithRepos(
		mocks.NewMockUserRepo(ctrl), wallets,
		mocks.NewMockCategoryRepo(ctrl), mocks.NewMockOperationRepo(ctrl),
	)
	h, err := New(svc, nil, web.FS, Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	userID := uuid.Must(uuid.NewV7())
	walletID := uuid.Must(uuid.NewV7())

	put := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/wallets/"+walletID.String(),
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("id", walletID.String())
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, userID))
		rec := httptest.NewRecorder()
		h.putWallet(rec, req)
		return rec
	}

	rec := put(url.Values{"name": {"Карта"}, "currency": {"RUBLES"}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("невалидная валюта: status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "валюты") {
		t.Errorf("невалидная валюта: тело %q без текста ошибки", rec.Body.String())
	}

	wallets.EXPECT().Update(gomock.Any(), walletID, userID, gomock.Any()).
		Return(nil, storage.ErrNotFound)
	if rec := put(url.Values{"name": {"Карта"}, "currency": {"RUB"}}); rec.Code != http.StatusNotFound {
		t.Errorf("чужой кошелёк: status = %d, want 404", rec.Code)
	}
}
