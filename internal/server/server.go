package server

import (
	"fmt"
	"log"
	"net/http"
	"store/internal"
	"store/internal/server/handler"
	"store/pkg/database"
)

func Run(cfg internal.Config) {
	fmt.Println("Starting db...")
	db := database.NewUserDatabase(cfg.Database.Path)
	defer db.Close()

	h := &handler.Handler{DB: db}

	http.HandleFunc("/add/", h.Add)
	http.HandleFunc("/add", h.AddDefault)

	fmt.Println("Server running at :" + cfg.Server.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Server.Port, nil))
}
