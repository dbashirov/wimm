package handler

import (
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
