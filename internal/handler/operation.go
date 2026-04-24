package handler

import (
	"errors"
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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

	walletID, err := uuid.Parse(r.FormValue("wallet_id"))
	if err != nil {
		http.Error(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}
	categoryID, err := uuid.Parse(r.FormValue("category_id"))
	if err != nil {
		http.Error(w, "invalid category_id", http.StatusBadRequest)
		return
	}
	opType := model.OperationType(r.FormValue("type"))
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil || amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")

	op, err := h.svc.CreateOperation(r.Context(), userID, walletID, categoryID, opType, amount, operatedAt, description)
	if err != nil {
		if errors.Is(err, service.ErrWrongCategoryType) {
			http.Error(w, "Тип категории не совпадает с типом операции", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fetch joined data for the partial
	full, err := h.svc.GetOperation(r.Context(), op.ID, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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
	walletID, err := uuid.Parse(r.FormValue("wallet_id"))
	if err != nil {
		http.Error(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}
	categoryID, err := uuid.Parse(r.FormValue("category_id"))
	if err != nil {
		http.Error(w, "invalid category_id", http.StatusBadRequest)
		return
	}
	opType := model.OperationType(r.FormValue("type"))
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil || amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")

	op, err := h.svc.UpdateOperation(r.Context(), id, userID, walletID, categoryID, opType, amount, operatedAt, description)
	if err != nil {
		if errors.Is(err, service.ErrWrongCategoryType) {
			http.Error(w, "Тип категории не совпадает с типом операции", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	full, err := h.svc.GetOperation(r.Context(), op.ID, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
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

	fromID, err := uuid.Parse(r.FormValue("from_wallet_id"))
	if err != nil {
		http.Error(w, "invalid from_wallet_id", http.StatusBadRequest)
		return
	}
	toID, err := uuid.Parse(r.FormValue("to_wallet_id"))
	if err != nil {
		http.Error(w, "invalid to_wallet_id", http.StatusBadRequest)
		return
	}
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil || amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")

	expense, income, err := h.svc.CreateTransfer(r.Context(), userID, fromID, toID, amount, operatedAt, description)
	if err != nil {
		if errors.Is(err, service.ErrSameWallet) {
			http.Error(w, "Нельзя переводить на тот же кошелёк", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
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

// parseDateTime парсит строку из datetime-local input формата "2006-01-02T15:04"
func parseDateTime(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02T15:04", s, time.Local)
}
