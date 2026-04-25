package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/dbashirov/wimm/internal/model"
)

type dashboardPageData struct {
	basePage
	*model.DashboardData
}

func (h *Handler) getDashboard(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	data, err := h.svc.DashboardData(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "DashboardData", err)
		return
	}
	h.render(w, "dashboard", dashboardPageData{DashboardData: data})
}

// getDashboardJSON returns dashboard data as JSON for Chart.js
func (h *Handler) getDashboardJSON(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	data, err := h.svc.DashboardData(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "DashboardData", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("dashboard JSON encode: %v", err)
	}
}
