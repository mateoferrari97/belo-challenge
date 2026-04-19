package web

import (
	"fmt"
	"net/http"
	"strings"
)

type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewHTTPError(status int, message string) *HTTPError {
	return NewHTTPErrorf(status, "%s", message)
}

func NewHTTPErrorf(status int, format string, args ...any) *HTTPError {
	return &HTTPError{
		Code:    strings.ReplaceAll(strings.ToLower(http.StatusText(status)), " ", "_"),
		Message: fmt.Sprintf(format, args...),
		Status:  status,
	}
}
