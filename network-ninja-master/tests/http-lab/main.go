package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	address := flag.String("addr", "127.0.0.1:18080", "loopback listener")
	flag.Parse()
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Network-Ninja-Test", "present")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNoContent)
	})
	log.Fatal(http.ListenAndServe(*address, handler))
}
