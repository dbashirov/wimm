package handler

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) getDashboard(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	data, err := h.svc.DashboardData(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.render(w, "dashboard", data)
}

// getDashboardJSON returns dashboard data as JSON for Chart.js
func (h *Handler) getDashboardJSON(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	data, err := h.svc.DashboardData(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
