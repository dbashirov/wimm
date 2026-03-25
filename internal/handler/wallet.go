package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/service"
)

type walletsData struct {
	basePage
	Wallets []model.Wallet
	Error   string
}

func (h *Handler) getWallets(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.render(w, "wallets", walletsData{Wallets: wallets})
}

func (h *Handler) postWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	name := r.FormValue("name")
	currency := r.FormValue("currency")

	wallet, err := h.svc.CreateWallet(r.Context(), userID, name, currency)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}

func (h *Handler) getWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	wallet, err := h.svc.GetWallet(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}

func (h *Handler) getWalletEdit(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	wallet, err := h.svc.GetWallet(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "wallets", "wallet-edit-row", wallet)
}

func (h *Handler) putWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	currency := r.FormValue("currency")

	wallet, err := h.svc.UpdateWallet(r.Context(), id, userID, name, currency)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}

func (h *Handler) deleteWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteWallet(r.Context(), id, userID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
