package main

import (
	"log"
	"net/http"

	"github.com/Albe83/id-service/internal/server"
)

func main() {
	srv := server.New()
	mux := srv.Routes()

	addr := ":8080"
	log.Printf("id-service listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
