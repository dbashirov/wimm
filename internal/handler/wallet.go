package handler

import (
	"net/http"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
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
	inp := model.WalletInput{
		Name:        r.FormValue("name"),
		Currency:    r.FormValue("currency"),
		Description: r.FormValue("description"),
	}
	wallet, err := h.svc.CreateWallet(r.Context(), userID, inp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}

func (h *Handler) getWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
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
	id, err := uuid.Parse(r.PathValue("id"))
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
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	inp := model.WalletInput{
		Name:        r.FormValue("name"),
		Currency:    r.FormValue("currency"),
		Description: r.FormValue("description"),
	}
	wallet, err := h.svc.UpdateWallet(r.Context(), id, userID, inp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}

func (h *Handler) deleteWallet(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteWallet(r.Context(), id, userID); err != nil {
		handleDeleteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
