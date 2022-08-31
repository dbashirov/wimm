package category

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"wimm/internal/domain/category/storage"
	"wimm/internal/handlers"
	"wimm/internal/middleware"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
)

const (
	categoriesURL = "/categories"
	categoryURL   = "/category/:id"
)

type handler struct {
	repository storage.Repository
}

func NewHandler(repository storage.Repository) handlers.Handler {
	return &handler{
		repository: repository,
	}
}

func (h *handler) Register(router *mux.Router) {
	router.HandleFunc(categoriesURL, middleware.Middleware(h.GetList)).Methods("GET")
	router.HandleFunc(categoryURL, middleware.Middleware(h.GetCategory)).Methods("GET")
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
