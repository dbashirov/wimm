package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
	"github.com/google/uuid"
)

type operationsData struct {
	basePage
	Operations []model.Operation
	Wallets    []model.Wallet
	Categories []model.Category
	Filter     model.OperationFilter
	Page       int
	Error      string
}

func (h *Handler) getOperations(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	q := r.URL.Query()

	filter := model.OperationFilter{UserID: userID}
	if id, err := uuid.Parse(q.Get("wallet_id")); err == nil {
		filter.WalletID = id
	}
	if id, err := uuid.Parse(q.Get("category_id")); err == nil {
		filter.CategoryID = id
	}
	filter.Type = model.OperationType(q.Get("type"))
	if s := q.Get("date_from"); s != "" {
		filter.DateFrom, _ = time.Parse("2006-01-02", s)
	}
	if s := q.Get("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.DateTo = t.AddDate(0, 0, 1) // начало следующего дня, фильтр в SQL: < DateTo
		}
	}
	filter.Page, _ = strconv.Atoi(q.Get("page"))
	if filter.Page < 1 {
		filter.Page = 1
	}

	ops, err := h.svc.ListOperations(r.Context(), filter)
	if err != nil {
		h.serverError(w, r, "ListOperations", err)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListWallets", err)
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListCategories", err)
		return
	}
	h.render(w, "operations", operationsData{
		Operations: ops,
		Wallets:    wallets,
		Categories: cats,
		Filter:     filter,
		Page:       filter.Page,
	})
}

func (h *Handler) postOperation(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())

	inp, err := parseOperationInput(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	op, err := h.svc.CreateOperation(r.Context(), userID, inp)
	if err != nil {
		if errors.Is(err, service.ErrWrongCategoryType) {
			http.Error(w, "Тип категории не совпадает с типом операции", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.serverError(w, r, "CreateOperation", err)
		return
	}

	full, err := h.svc.GetOperation(r.Context(), op.ID, userID)
	if err != nil {
		h.serverError(w, r, "GetOperation", err)
		return
	}
	h.renderPartial(w, "operations", "operation-row", full)
}

func (h *Handler) getOperationRow(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	op, err := h.svc.GetOperation(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "operations", "operation-row", op)
}

func (h *Handler) getOperationEdit(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	op, err := h.svc.GetOperation(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if op.TransferID != nil {
		http.Error(w, "переводы нельзя редактировать", http.StatusBadRequest)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListWallets", err)
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListCategories", err)
		return
	}
	h.renderPartial(w, "operations", "operation-edit-row", map[string]any{
		"Op":         op,
		"Wallets":    wallets,
		"Categories": cats,
	})
}

func (h *Handler) putOperation(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	inp, err := parseOperationInput(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	op, err := h.svc.UpdateOperation(r.Context(), id, userID, inp)
	if err != nil {
		if errors.Is(err, service.ErrWrongCategoryType) {
			http.Error(w, "Тип категории не совпадает с типом операции", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.serverError(w, r, "UpdateOperation", err)
		return
	}

	full, err := h.svc.GetOperation(r.Context(), op.ID, userID)
	if err != nil {
		h.serverError(w, r, "GetOperation", err)
		return
	}
	h.renderPartial(w, "operations", "operation-row", full)
}

func (h *Handler) deleteOperation(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteOperation(r.Context(), id, userID); err != nil {
		handleDeleteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) postTransfer(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())

	inp, err := parseTransferInput(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expense, income, err := h.svc.CreateTransfer(r.Context(), userID, inp)
	if err != nil {
		if errors.Is(err, service.ErrSameWallet) {
			http.Error(w, "Нельзя переводить на тот же кошелёк", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.serverError(w, r, "CreateTransfer", err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	h.renderPartial(w, "operations", "operation-row", expense)
	h.renderPartial(w, "operations", "operation-row", income)
}

func (h *Handler) getTransferRows(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	transferID, err := uuid.Parse(r.PathValue("transfer_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	expense, income, err := h.svc.GetTransfer(r.Context(), transferID, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	h.renderPartial(w, "operations", "operation-row", expense)
	h.renderPartial(w, "operations", "operation-row", income)
}

func (h *Handler) getTransferEdit(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	transferID, err := uuid.Parse(r.PathValue("transfer_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	expense, income, err := h.svc.GetTransfer(r.Context(), transferID, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListWallets", err)
		return
	}
	h.renderPartial(w, "operations", "transfer-edit-row", map[string]any{
		"Expense": expense,
		"Income":  income,
		"Wallets": wallets,
	})
}

func (h *Handler) putTransfer(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	transferID, err := uuid.Parse(r.PathValue("transfer_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	inp, err := parseTransferInput(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	expense, income, err := h.svc.UpdateTransfer(r.Context(), transferID, userID, inp)
	if err != nil {
		if errors.Is(err, service.ErrSameWallet) {
			http.Error(w, "Нельзя переводить на тот же кошелёк", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.serverError(w, r, "UpdateTransfer", err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	h.renderPartial(w, "operations", "operation-row", expense)
	h.renderPartial(w, "operations", "operation-row", income)
}

func (h *Handler) deleteTransfer(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	transferID, err := uuid.Parse(r.PathValue("transfer_id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteTransfer(r.Context(), transferID, userID); err != nil {
		handleDeleteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func parseOperationInput(r *http.Request) (model.OperationInput, error) {
	walletID, err := uuid.Parse(r.FormValue("wallet_id"))
	if err != nil {
		return model.OperationInput{}, fmt.Errorf("invalid wallet_id")
	}
	categoryID, err := uuid.Parse(r.FormValue("category_id"))
	if err != nil {
		return model.OperationInput{}, fmt.Errorf("invalid category_id")
	}
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil || amount <= 0 {
		return model.OperationInput{}, fmt.Errorf("invalid amount")
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		return model.OperationInput{}, fmt.Errorf("invalid date")
	}
	return model.OperationInput{
		WalletID:    walletID,
		CategoryID:  categoryID,
		Type:        model.OperationType(r.FormValue("type")),
		Amount:      amount,
		OperatedAt:  operatedAt,
		Description: r.FormValue("description"),
	}, nil
}

func parseTransferInput(r *http.Request) (model.TransferInput, error) {
	fromID, err := uuid.Parse(r.FormValue("from_wallet_id"))
	if err != nil {
		return model.TransferInput{}, fmt.Errorf("invalid from_wallet_id")
	}
	toID, err := uuid.Parse(r.FormValue("to_wallet_id"))
	if err != nil {
		return model.TransferInput{}, fmt.Errorf("invalid to_wallet_id")
	}
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil || amount <= 0 {
		return model.TransferInput{}, fmt.Errorf("invalid amount")
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		return model.TransferInput{}, fmt.Errorf("invalid date")
	}
	return model.TransferInput{
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       amount,
		OperatedAt:   operatedAt,
		Description:  r.FormValue("description"),
	}, nil
}

// parseDateTime парсит строку из datetime-local input формата "2006-01-02T15:04".
// Время интерпретируется в UTC — это даёт детерминированное поведение
// независимо от TZ-настройки контейнера. Frontend (flatpickr) отображает
// значения как локальные, что для личного финансового учёта приемлемо;
// если в будущем понадобится корректная мультизона — клиент должен
// передавать TZ явно.
func parseDateTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04", s)
}
