package main

import (
	"log"
	"net/http"
)

func main() {
	addr := ":" + envOr("PORT", "8082")
	handler, err := runServer(addr)
	if err != nil {
		log.Fatal(err)
	}
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
