package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Param(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func Decode(r *http.Request, v any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}

	return nil
}
