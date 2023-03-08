package user

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"wimm/internal/handlers"
	"wimm/internal/middleware"
	"wimm/internal/model"

	// "wimm/internal/store"

	// "github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

const (
	usersURL     = "/users"
	userURL      = "/users/{id:[0-9]+}"
	userEmailURL = "/users/{email}"
)

type handler struct {
	repository model.UserRepository
}

func NewHandler(repository model.UserRepository) handlers.Handler {
	return &handler{
		repository: repository,
	}
}

func (h *handler) Register(router *mux.Router) {
	router.HandleFunc(usersURL, middleware.Middleware(h.GetList)).Methods(http.MethodGet)
	router.HandleFunc(usersURL, middleware.Middleware(h.CreateUser)).Methods(http.MethodPost)
	router.HandleFunc(userURL, middleware.Middleware(h.Find)).Methods(http.MethodGet)
	router.HandleFunc(userEmailURL, middleware.Middleware(h.FindByEmail)).Methods(http.MethodGet)
}

// type message struct {
// 	Status string `json:"status"`
// }

func (h *handler) GetList(w http.ResponseWriter, r *http.Request) error {
	users, err := h.repository.GetAll(context.TODO())
	if err != nil {
		handlers.Error(w, r, http.StatusBadRequest, err)
		return err
	}
	// test+
	// log.Println(r.Body)
	// var m message
	// err = json.NewDecoder(r.Body).Decode(&m)
	// if err != nil {
	// 	log.Println("Decode body error")
	// }
	// // err = json.Unmarshal([]byte(r.Body), m)
	// log.Println(m)

	// allM, err := json.Marshal(token)
	// if err != nil {
	// 	log.Println("Encode body error")
	// }

	// w.Write(allM)

	// JWT
	// token := jwt.New(jwt.SigningMethodEdDSA)
	// // tokenString, err := token.SignedString("JWTTSecretKey")
	// // if err != nil {
	// // 	return err
	// // }
	// allM, err := json.Marshal(token)
	// if err != nil {
	// 	return err
	// }
	// w.Write(allM)
	// test-

	handlers.Respond(w, r, http.StatusOK, users)

	return nil
}

func (h *handler) Find(w http.ResponseWriter, r *http.Request) error {

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		handlers.Error(w, r, http.StatusBadRequest, err)
		return err
	}

	u, err := h.repository.Find(context.TODO(), id)
	if err != nil {
		handlers.Error(w, r, http.StatusBadRequest, err)
		return err
	}

	handlers.Respond(w, r, http.StatusOK, u)

	return nil
}

func (h *handler) FindByEmail(w http.ResponseWriter, r *http.Request) error {

	vars := mux.Vars(r)
	email := vars["email"]
	if email == "" {
		handlers.Error(w, r, http.StatusBadRequest, fmt.Errorf("undefined parameter <email>"))
		return fmt.Errorf("undefined parameter <email>")
	}

	u, err := h.repository.FindByEmail(context.TODO(), email)
	if err != nil {
		handlers.Error(w, r, http.StatusBadRequest, err)
		return err
	}

	handlers.Respond(w, r, http.StatusOK, u)

	return nil
}

func (h *handler) CreateUser(w http.ResponseWriter, r *http.Request) error {

	var u model.User

	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		handlers.Error(w, r, http.StatusBadRequest, err)
		return err
	}
	if err := h.repository.Create(r.Context(), u); err != nil {
		log.Println("User creation error")
		handlers.Error(w, r, http.StatusUnprocessableEntity, err)
		return err
	}

	u.Cleaning()

	handlers.Respond(w, r, http.StatusCreated, u)

	return nil
}
