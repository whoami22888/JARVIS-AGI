package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	address := flag.String("addr", "127.0.0.1:18080", "loopback listener")
	flag.Parse()
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Network-Ninja-Test", "present")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Addr: *address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	log.Fatal(server.ListenAndServe())
}
