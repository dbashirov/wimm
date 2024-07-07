package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
	"wimm/config"
	"wimm/internal/category"
	categoryStorage "wimm/internal/category/storage"
	"wimm/internal/handlers"
	"wimm/internal/user"
	userStorage "wimm/internal/user/storage"
	"wimm/pkg/client/postgresql"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	sessionName = "wimm"
)

var (
	errIncorrectEmailOrPassword = errors.New("incorrect email or password")
	errNotAuthenticated         = errors.New("not authenticated")
)

type App struct {
	cfg          *config.Config
	pgClient     *pgxpool.Pool
	router       *mux.Router
	httpServer   *http.Server
	sessionStore sessions.Store
}

func NewApp(cfg *config.Config) (*App, error) {

	log.Println("router initializing")

	router := mux.NewRouter()

	pgClient, err := postgresql.NewClient(context.TODO(), cfg.Storage, 5)
	if err != nil {
		log.Fatal(err)
	}
	// TODO: gracefulshutdown
	// defer pool.Close()

	sessionStore := sessions.NewCookieStore([]byte(cfg.Session.SessionKey))

	a := &App{
		cfg:          cfg,
		pgClient:     pgClient,
		router:       router,
		sessionStore: sessionStore,
	}

	a.configureRouter()

	return a, nil
}

func (a *App) Run() {
	a.StartHTTP()
}

func (a *App) StartHTTP() {

	// Добавляем тестовые данные
	// addTestData(userRepository, categoryRepository)

	listener, listenErr := net.Listen("tcp", a.cfg.Server.Port)
	if listenErr != nil {
		log.Fatal(listenErr)
	}

	a.httpServer = &http.Server{
		Handler:      a.router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	err := a.httpServer.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("application completely initialized and started")
}

func (a *App) configureRouter() {

	// Registering user handlers
	userRepository := userStorage.NewRepository(a.pgClient)
	userHandler := user.NewHandler(userRepository)
	userHandler.Register(a.router)

	// Категории
	categoryRepository := categoryStorage.NewRepository(a.pgClient)
	categoryHandler := category.NewHandler(categoryRepository)
	categoryHandler.Register(a.router)
}

func (a *App) handleSessionsCreate() http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			handlers.Error(w, r, http.StatusBadRequest, err)
			return
		}

		userRepository := userStorage.NewRepository(a.pgClient)
		u, err := userRepository.FindByEmail(context.Background(), req.Email)
		if err != nil || !u.ComparePawwword(req.Password) {
			handlers.Error(w, r, http.StatusUnauthorized, errIncorrectEmailOrPassword)
			return
		}

		session, err := a.sessionStore.Get(r, sessionName)
		if err != nil {
			handlers.Error(w, r, http.StatusInternalServerError, err)
			return
		}

		session.Values["user_id"] = u.ID
		if err := a.sessionStore.Save(r, w, session); err != nil {
			handlers.Error(w, r, http.StatusInternalServerError, err)
			return
		}

		handlers.Respond(w, r, http.StatusOK, nil)
	}
}
