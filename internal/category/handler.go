package category

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"wimm/internal/handlers"
	"wimm/internal/middleware"
	"wimm/internal/model"

	"github.com/gorilla/mux"
)

const (
	categoriesURL = "/categories"
	categoryURL   = "/category/{id:[0-9]+}"
)

type handler struct {
	repository model.CategoryRepository
}

func NewHandler(repository model.CategoryRepository) handlers.Handler {
	return &handler{
		repository: repository,
	}
}

func (h *handler) Register(router *mux.Router) {
	router.HandleFunc(categoriesURL, middleware.Middleware(h.GetList)).Methods(http.MethodGet)
	router.HandleFunc(categoryURL, middleware.Middleware(h.GetCategory)).Methods(http.MethodGet)
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

	vars := mux.Vars(r)
	idStr := vars["id"]
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
