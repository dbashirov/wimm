package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type Handler interface {
	Register(router *mux.Router)
}

func Error(w http.ResponseWriter, r *http.Request, code int, err error) {
	Respond(w, r, code, map[string]string{"error": err.Error()})
}

func Respond(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
