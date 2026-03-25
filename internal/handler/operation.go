package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
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
	filter.WalletID, _ = strconv.ParseInt(q.Get("wallet_id"), 10, 64)
	filter.CategoryID, _ = strconv.ParseInt(q.Get("category_id"), 10, 64)
	filter.Type = q.Get("type")
	if s := q.Get("date_from"); s != "" {
		filter.DateFrom, _ = time.Parse("2006-01-02", s)
	}
	if s := q.Get("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.DateTo = t.Add(24*time.Hour - time.Second)
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

	walletID, err := strconv.ParseInt(r.FormValue("wallet_id"), 10, 64)
	if err != nil || walletID <= 0 {
		http.Error(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}
	categoryID, err := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	if err != nil || categoryID <= 0 {
		http.Error(w, "invalid category_id", http.StatusBadRequest)
		return
	}
	opType := r.FormValue("type")
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	op, err := h.svc.GetOperation(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	walletID, err := strconv.ParseInt(r.FormValue("wallet_id"), 10, 64)
	if err != nil || walletID <= 0 {
		http.Error(w, "invalid wallet_id", http.StatusBadRequest)
		return
	}
	categoryID, err := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	if err != nil || categoryID <= 0 {
		http.Error(w, "invalid category_id", http.StatusBadRequest)
		return
	}
	opType := r.FormValue("type")
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteOperation(r.Context(), id, userID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseDateTime парсит строку из datetime-local input формата "2006-01-02T15:04"
func parseDateTime(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02T15:04", s, time.Local)
}
