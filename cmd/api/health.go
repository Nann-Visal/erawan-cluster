package main

import "net/http"

func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ok(w, "Go erawan-cluster API is healthy", map[string]any{"service": "erawan-cluster"})
}
