package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type envelope map[string]any

func writeJSON(w http.ResponseWriter, status int, payload envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func ok(w http.ResponseWriter, message string, data any) {
	body := envelope{
		"status":  "ok",
		"message": message,
	}
	if data != nil {
		body["data"] = data
	}
	writeJSON(w, http.StatusOK, body)
}

func errJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, envelope{
		"status":  "error",
		"message": message,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	return nil
}
