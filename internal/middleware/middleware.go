package middleware

import (
	"fmt"
	"net/http"
)

type appHandler func(w http.ResponseWriter, r *http.Request) error

func Middleware(h appHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := h(w, r)
		if err != nil {
			fmt.Printf("Error handle: %s\n", err)
		}
	}
}
