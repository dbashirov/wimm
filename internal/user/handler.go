package user

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"wimm/internal/handlers"
	"wimm/internal/middleware"

	// "wimm/internal/store"

	"github.com/julienschmidt/httprouter"
)

const (
	usersURL     = "/users"
	userURL      = "/users/:id"
	userEmailURL = "/useremail/:email"
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
	router.HandlerFunc(http.MethodGet, userURL, middleware.Middleware(h.Find))
	router.HandlerFunc(http.MethodGet, userEmailURL, middleware.Middleware(h.FindByEmail))
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

func (h *handler) Find(w http.ResponseWriter, r *http.Request) error {

	params := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	idStr := params.ByName("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	u, err := h.repository.Find(context.TODO(), id)
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	allBytes, err := json.Marshal(u)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(allBytes)

	return nil
}

func (h *handler) FindByEmail(w http.ResponseWriter, r *http.Request) error {

	params := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	email := params.ByName("email")
	if email == "" {
		w.WriteHeader(400)
		return fmt.Errorf("undefined parameter <email>")
	}

	u, err := h.repository.FindByEmail(context.TODO(), email)
	if err != nil {
		w.WriteHeader(400)
		return err
	}

	allBytes, err := json.Marshal(u)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(allBytes)

	return nil
}
