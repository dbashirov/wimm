package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/dbashirov/wimm/internal/storage/mocks"
	"github.com/dbashirov/wimm/web"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// TestPutCategoryInUse - смена типа категории с операциями: 409 с пояснением.
func TestPutCategoryInUse(t *testing.T) {
	ctrl := gomock.NewController(t)
	cats := mocks.NewMockCategoryRepo(ctrl)
	svc := service.NewWithRepos(
		mocks.NewMockUserRepo(ctrl), mocks.NewMockWalletRepo(ctrl),
		cats, mocks.NewMockOperationRepo(ctrl),
	)
	h, err := New(svc, nil, web.FS, Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	userID := uuid.Must(uuid.NewV7())
	catID := uuid.Must(uuid.NewV7())
	cats.EXPECT().Update(gomock.Any(), catID, userID, "Еда", model.TypeIncome).
		Return(nil, storage.ErrCategoryInUse)

	form := url.Values{"name": {"Еда"}, "type": {"income"}}
	req := httptest.NewRequest(http.MethodPut, "/categories/"+catID.String(),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", catID.String())
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, userID))
	rec := httptest.NewRecorder()
	h.putCategory(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

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
