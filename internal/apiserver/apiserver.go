package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
	config "wimm/configs"
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

	userRepository := user.NewRepository(pool)
	router := httprouter.New()

	// users, err := userRepository.GetAll(context.TODO())
	// if err != nil {
	// 	return err
	// }

	userHandler := user2.NewHandler(userRepository)
	userHandler.Register(router)

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
