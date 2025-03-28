package server

import (
	"fmt"
	"net/http"
)

func Start() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "🦒 Welcome to GiraffeCloud!")
	})
	return http.ListenAndServe(":8080", nil)
}