package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
	config "wimm/configs"
	category2 "wimm/internal/category"
	category "wimm/internal/category/repository"
	user2 "wimm/internal/user"
	user "wimm/internal/user/repository"
	"wimm/pkg/client/postgresql"

	"github.com/julienschmidt/httprouter"
)

func Start(cfg *config.Config) {

	pool, err := postgresql.NewClient(context.TODO(), cfg.Storage, 5)
	if err != nil {
		return
	}
	defer pool.Close()

	router := httprouter.New()

	// Работа с пользователями
	userRepository := user.NewRepository(pool)
	userHandler := user2.NewHandler(userRepository)
	userHandler.Register(router)

	// Категории
	categoryRepository := category.NewRepository(pool)
	categoryHandler := category2.NewHandler(categoryRepository)
	categoryHandler.Register(router)

	// Запуск сервера
	listener, listenErr := net.Listen("tcp", cfg.Server.Port)
	if listenErr != nil {
		fmt.Println(listenErr)
	}

	server := &http.Server{
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	server.Serve(listener)

	// store := sqlstore.New(pool)
	// srv := newServer(store)

	// return http.ListenAndServe(cfg.Server.Port, srv)

}
