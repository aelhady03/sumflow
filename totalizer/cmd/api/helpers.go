package main

import (
	"encoding/json"
	"maps"
	"net/http"
)

// envelope is a custom type for a generic JSON object.
type envelope map[string]any

// writeJSON is a helper method for writing JSON responses to the client.
func (app *application) writeJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	maps.Copy(w.Header(), headers)

	w.WriteHeader(status)
	w.Write(js)
	return nil
}
