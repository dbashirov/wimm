package handler

import (
	"encoding/json"
	"net/http"
	"slices"

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
		h.serverError(w, r, "ListWallets", err)
		return
	}
	slices.SortStableFunc(wallets, func(a, b model.Wallet) int {
		if a.Hidden == b.Hidden {
			return 0
		}
		if !a.Hidden {
			return -1
		}
		return 1
	})
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
		h.serverError(w, r, "CreateWallet", err)
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
		h.serverError(w, r, "UpdateWallet", err)
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

func (h *Handler) postWalletsReorder(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	var body struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := h.svc.ReorderWallets(r.Context(), userID, body.IDs); err != nil {
		h.serverError(w, r, "ReorderWallets", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) postWalletSetDefault(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := h.svc.SetDefaultWallet(r.Context(), id, userID); err != nil {
		http.NotFound(w, r)
		return
	}
	wallets, err := h.svc.ListWallets(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "SetDefault-ListWallets", err)
		return
	}
	slices.SortStableFunc(wallets, func(a, b model.Wallet) int {
		if a.Hidden == b.Hidden {
			return 0
		}
		if !a.Hidden {
			return -1
		}
		return 1
	})
	w.Header().Set("Content-Type", "text/html")
	for _, wallet := range wallets {
		h.renderPartial(w, "wallets", "wallet-row", wallet)
	}
}

func (h *Handler) postWalletToggleHidden(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	wallet, err := h.svc.ToggleWalletHidden(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "wallets", "wallet-row", wallet)
}
