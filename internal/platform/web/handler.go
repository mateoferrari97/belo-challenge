package web

import (
	"errors"
	"log"
	"net/http"
)

type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		log.Printf("error : %+v", err)

		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			httpErr = NewHTTPErrorf(http.StatusInternalServerError, "%s", err.Error())
		}

		if err = Respond(w, httpErr, httpErr.Status); err != nil {
			log.Printf("writing http response : %v", err)
		}
	}
}
