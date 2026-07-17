package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxRequestBodySize = 1 << 20

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var syntaxError *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("request body contains malformed JSON at position %d", syntaxError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("request body must not be empty")
		case errors.As(err, &typeError):
			if typeError.Field != "" {
				return fmt.Errorf("request field %q has an invalid value", typeError.Field)
			}
			return errors.New("request body contains an invalid value")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return fmt.Errorf("request body contains %s", err.Error()[len("json: "):])
		case err.Error() == "http: request body too large":
			return fmt.Errorf("request body must not exceed %d bytes", maxRequestBodySize)
		default:
			return errors.New("request body contains invalid JSON")
		}
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON value")
	}
	return nil
}

func WriteAPIError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "an internal server error occurred"

	switch {
	case errors.Is(err, ErrValidation):
		status = http.StatusBadRequest
		code = "validation_error"
		message = err.Error()
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "unauthorized"
		message = err.Error()
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
		message = err.Error()
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "resource not found"
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
		code = "conflict"
		message = err.Error()
	}

	WriteJSON(w, status, ErrorEnvelope{Error: APIError{Code: code, Message: message}})
}

func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusBadRequest, ErrorEnvelope{Error: APIError{
		Code: "bad_request", Message: message,
	}})
}

func PathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		WriteBadRequest(w, "path ID must be a positive integer")
		return 0, false
	}
	return id, true
}
