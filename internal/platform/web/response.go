package web

import (
	"encoding/json"
	"net/http"
)

func Respond(w http.ResponseWriter, v any, status int) error {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return nil
	}

	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(jsonData); err != nil {
		return err
	}

	return nil
}
