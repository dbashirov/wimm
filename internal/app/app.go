package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"
	"wimm/config"
	category2 "wimm/internal/domain/category"
	modelCategory "wimm/internal/domain/category/model"
	category "wimm/internal/domain/category/storage"
	"wimm/internal/domain/types/model"
	user2 "wimm/internal/domain/user"
	modelUser "wimm/internal/domain/user/model"
	user "wimm/internal/domain/user/storage"
	"wimm/pkg/client/postgresql"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
)

type App struct {
	cfg        *config.Config
	pgClient   *pgxpool.Pool
	router     *mux.Router
	httpServer *http.Server
}

func NewApp(cfg *config.Config) (App, error) {

	log.Println("router initializing")
	// router := httprouter.New()
	router := mux.NewRouter()

	pgClient, err := postgresql.NewClient(context.TODO(), cfg.Storage, 5)
	if err != nil {
		log.Fatal(err)
	}
	// defer pool.Close()

	return App{
		cfg:      cfg,
		pgClient: pgClient,
		router:   router,
	}, nil
}

func (a *App) Run() {
	a.StartHTTP()
}

func (a *App) StartHTTP() {

	// Работа с пользователями
	userRepository := user.NewRepository(a.pgClient)
	userHandler := user2.NewHandler(userRepository)
	userHandler.Register(a.router)

	// Категории
	categoryRepository := category.NewRepository(a.pgClient)
	categoryHandler := category2.NewHandler(categoryRepository)
	categoryHandler.Register(a.router)

	// Добавляем тестовые данные
	// addTestData(userRepository, categoryRepository)

	// Запуск сервера
	listener, listenErr := net.Listen("tcp", a.cfg.Server.Port)
	if listenErr != nil {
		log.Fatal(listenErr)
	}

	a.httpServer = &http.Server{
		Handler:      a.router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("application completely initialized and started")

	a.httpServer.Serve(listener)

}

func addTestData(ur user.Repository, cr category.Repository) {

	// Создаем пользователья
	u := modelUser.User{
		Username: "user3",
		Email:    "user3@mail.com",
		Password: "qweasd",
	}
	err := ur.Create(context.TODO(), u)
	if err != nil {
		log.Printf("User creation error: %s\n", err)
		return
	}

	// Создаем категорию
	c := modelCategory.Category{
		Title: "Тест 3",
		User:  u,
		Type:  model.TypeExpense,
	}
	err = cr.Create(context.TODO(), &c)
	if err != nil {
		log.Printf("Category creation error: %s\n", err)
	}
}
