package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	HasFilter  bool
	Page       int
	PrevURL    string
	NextURL    string
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
		filter.DateFrom, _ = time.Parse(dateLayout, s)
	}
	if s := q.Get("date_to"); s != "" {
		if parsed, err := time.Parse(dateLayout, s); err == nil {
			filter.DateTo = parsed.AddDate(0, 0, 1) // начало следующего дня, фильтр в SQL: < DateTo
		}
	}
	filter.Page = parsePage(q.Get("page"))

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
	hasFilter := filter.WalletID != uuid.Nil ||
		filter.CategoryID != uuid.Nil ||
		filter.Type != "" ||
		!filter.DateFrom.IsZero() ||
		!filter.DateTo.IsZero()

	h.render(w, "operations", operationsData{
		Operations: ops,
		Wallets:    wallets,
		Categories: cats,
		Filter:     filter,
		HasFilter:  hasFilter,
		Page:       filter.Page,
		PrevURL:    pageURL(q, filter.Page-1),
		NextURL:    nextPageURL(q, filter.Page, len(ops)),
	})
}

// pageURL - ссылка на страницу с сохранением фильтра из query; "" если
// страницы нет.
func pageURL(q url.Values, page int) string {
	if page < 1 || page > maxOperationsPage {
		return ""
	}
	next := url.Values{}
	for k, v := range q {
		next[k] = v
	}
	next.Set("page", strconv.Itoa(page))
	return "?" + next.Encode()
}

// nextPageURL - полная страница значит, что дальше могут быть операции.
func nextPageURL(q url.Values, page, count int) string {
	if count < model.OperationsPageSize {
		return ""
	}
	return pageURL(q, page+1)
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
		if errors.Is(err, service.ErrToAmountRequired) {
			http.Error(w, "Кошельки в разных валютах: укажите сумму зачисления",
				http.StatusBadRequest)
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
		if errors.Is(err, service.ErrToAmountRequired) {
			http.Error(w, "Кошельки в разных валютах: укажите сумму зачисления",
				http.StatusBadRequest)
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

// maxOperationsPage - 500 тысяч операций при 50 на странице. Без границы
// page=MaxInt переполняет расчёт OFFSET, и запрос падает с 500.
const maxOperationsPage = 10000

// parsePage разбирает номер страницы: мусор и значения < 1 - первая страница.
func parsePage(s string) int {
	page, err := strconv.Atoi(s)
	if err != nil || page < 1 {
		return 1
	}
	return min(page, maxOperationsPage)
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
	amount, err := parsePositiveMoney(r.FormValue("amount"))
	if err != nil {
		return model.OperationInput{}, err
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
	amount, err := parsePositiveMoney(r.FormValue("amount"))
	if err != nil {
		return model.TransferInput{}, err
	}
	// to_amount заполняется только при переводе между валютами
	var toAmount model.Money
	if raw := strings.TrimSpace(r.FormValue("to_amount")); raw != "" {
		toAmount, err = parsePositiveMoney(raw)
		if err != nil {
			return model.TransferInput{}, fmt.Errorf("invalid to_amount")
		}
	}
	operatedAt, err := parseDateTime(r.FormValue("operated_at"))
	if err != nil {
		return model.TransferInput{}, fmt.Errorf("invalid date")
	}
	return model.TransferInput{
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       amount,
		ToAmount:     toAmount,
		OperatedAt:   operatedAt,
		Description:  r.FormValue("description"),
	}, nil
}

func parsePositiveMoney(s string) (model.Money, error) {
	amount, err := model.ParseMoney(s)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	return amount, nil
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
