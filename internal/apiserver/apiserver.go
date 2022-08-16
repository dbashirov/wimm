package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
	config "wimm/configs"
	"wimm/pkg/client/postgresql"

	"github.com/julienschmidt/httprouter"
)

func Start(cfg *config.Config) error {

	pool, err := postgresql.NewClient(context.TODO(), cfg.Storage, 5)
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := store.NewRepository(pool)
	router := httprouter.New()

	users, err := repository.GetAll(context.TODO())
	if err != nil {
		return err
	}

	userHandler := user2.NewHandler(repository)
	userHandler.Register(router)

	listener, listenErr := net.Listen("tcp", ":8080")
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
