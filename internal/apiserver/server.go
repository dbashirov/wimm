package apiserver

// import (
// 	"context"
// 	"encoding/json"
// 	"net/http"
// 	"wimm/internal/store"

// 	"github.com/gorilla/mux"
// 	"github.com/sirupsen/logrus"
// )

// type server struct {
// 	router *mux.Router
// 	logger *logrus.Logger
// 	store  store.Store
// }

// func newServer(store store.Store) *server {
// 	s := &server{
// 		router: mux.NewRouter(),
// 		logger: logrus.New(),
// 		store:  store,
// 	}
// 	s.configureRouter()
// 	return s
// }

// func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	s.router.ServeHTTP(w, r)
// }

// func (s *server) configureRouter() {
// 	s.router.HandleFunc("/users", s.GetListUsers()).Methods("GET")
// }

// func (s *server) error(w http.ResponseWriter, r *http.Request, code int, err error) {
// 	s.respond(w, r, code, map[string]string{"error": err.Error()})
// }

// func (s *server) respond(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
// 	w.WriteHeader(code)
// 	if data != nil {
// 		json.NewEncoder(w).Encode(data)
// 	}
// }

// func (s *server) GetListUsers() http.HandlerFunc {

// 	return func(w http.ResponseWriter, r *http.Request) {

// 		users, err := s.store.User().GetAll(context.TODO())
// 		if err != nil {
// 			s.error(w, r, http.StatusBadRequest, err)
// 			return
// 		}

// 		allBytes, err := json.Marshal(users)
// 		if err != nil {
// 			s.error(w, r, http.StatusBadRequest, err)
// 			return
// 		}

// 		w.WriteHeader(http.StatusOK)
// 		w.Write(allBytes)

// 	}
// }
