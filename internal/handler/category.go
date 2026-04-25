package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
)

type categoriesData struct {
	basePage
	Income  []model.Category
	Expense []model.Category
	Error   string
}

func (h *Handler) getCategories(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	cats, err := h.svc.ListCategories(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "ListCategories", err)
		return
	}
	data := categoriesData{}
	for _, c := range cats {
		if c.Type == model.TypeIncome {
			data.Income = append(data.Income, c)
		} else {
			data.Expense = append(data.Expense, c)
		}
	}
	h.render(w, "categories", data)
}

func (h *Handler) postCategory(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	inp := model.CategoryInput{
		Name:        r.FormValue("name"),
		Type:        model.OperationType(r.FormValue("type")),
		Description: r.FormValue("description"),
	}
	cat, err := h.svc.CreateCategory(r.Context(), userID, inp)
	if err != nil {
		h.serverError(w, r, "CreateCategory", err)
		return
	}
	h.renderPartial(w, "categories", "category-row", cat)
}

func (h *Handler) getCategory(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cat, err := h.svc.GetCategory(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "categories", "category-row", cat)
}

func (h *Handler) getCategoryEdit(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	cat, err := h.svc.GetCategory(r.Context(), id, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.renderPartial(w, "categories", "category-edit-row", cat)
}

func (h *Handler) putCategory(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := r.FormValue("name")
	description := r.FormValue("description")
	cat, err := h.svc.UpdateCategory(r.Context(), id, userID, name, description)
	if err != nil {
		h.serverError(w, r, "UpdateCategory", err)
		return
	}
	h.renderPartial(w, "categories", "category-row", cat)
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), id, userID); err != nil {
		handleDeleteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getCategoryOptions returns <option> tags for a given type (for htmx dynamic select)
func (h *Handler) getCategoryOptions(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	catType := model.OperationType(r.URL.Query().Get("type"))
	if catType == "" {
		catType = model.TypeExpense
	}
	cats, err := h.svc.ListCategoriesByType(r.Context(), userID, catType)
	if err != nil {
		h.serverError(w, r, "ListCategoriesByType", err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	for _, c := range cats {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, c.ID, template.HTMLEscapeString(c.Name))
	}
}
