package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
	"wimm/config"
	category2 "wimm/internal/category"
	category "wimm/internal/category/repository"
	"wimm/internal/model"
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

	// Добавляем тестовые данные
	// addTestData(userRepository, categoryRepository)

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

}

func addTestData(ur user2.Repository, cr category2.Repository) {

	// Создаем пользователья
	u := model.User{
		Username: "user3",
		Email:    "user3@mail.com",
		Password: "qweasd",
	}
	err := ur.Create(context.TODO(), u)
	if err != nil {
		fmt.Printf("User creation error: %s\n", err)
		return
	}

	// Создаем категорию
	c := model.Category{
		Title: "Тест 3",
		User:  u,
		Type:  model.TypeExpense,
	}
	err = cr.Create(context.TODO(), &c)
	if err != nil {
		fmt.Printf("Category creation error: %s\n", err)
	}
}
