package category

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"wimm/internal/handlers"
	"wimm/internal/middleware"

	"github.com/julienschmidt/httprouter"
)

const (
	categoriesURL = "/categories"
	categoryURL   = "/category/:id"
)

type handler struct {
	repository Repository
}

func NewHandler(repository Repository) handlers.Handler {
	return &handler{
		repository: repository,
	}
}

func (h *handler) Register(router *httprouter.Router) {
	router.HandlerFunc(http.MethodGet, categoriesURL, middleware.Middleware(h.GetList))
	router.HandlerFunc(http.MethodGet, categoryURL, middleware.Middleware(h.GetCategory))
}

func (h *handler) GetList(w http.ResponseWriter, r *http.Request) error {
	categories, err := h.repository.GetAll(context.TODO())
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	allBytes, err := json.Marshal(categories)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(allBytes)

	return nil
}

func (h *handler) GetCategory(w http.ResponseWriter, r *http.Request) error {

	params := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	idStr := params.ByName("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(400)
		return err
	}
	category, err := h.repository.Find(context.TODO(), id)
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	allBytes, err := json.Marshal(category)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(allBytes)

	return nil
}
