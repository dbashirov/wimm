package user

import (
	"context"
	"encoding/json"
	"net/http"
	"wimm/internal/handlers"
	"wimm/internal/middleware"
	// "wimm/internal/store"

	"github.com/julienschmidt/httprouter"
)

const (
	usersURL = "/users"
	userURL  = "/user/:uuid"
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
	router.HandlerFunc(http.MethodGet, usersURL, middleware.Middleware(h.GetList))
}

func (h *handler) GetList(w http.ResponseWriter, r *http.Request) error {
	users, err := h.repository.GetAll(context.TODO())
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	allBytes, err := json.Marshal(users)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(allBytes)

	return nil
}
