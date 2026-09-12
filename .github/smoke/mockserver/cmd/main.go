package main

import (
	"log"
	"net/http"
	"time"

	"one-api/.github/smoke/mockserver"
)

func main() {
	handler := mockserver.New()
	server := func(address string) *http.Server {
		return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
	}
	go func() { log.Fatal(server(":8443").ListenAndServeTLS("/fixture/server.crt", "/fixture/server.key")) }()
	log.Fatal(server(":8000").ListenAndServe())
}
