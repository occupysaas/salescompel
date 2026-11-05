package contract

import "net/http"

type APIError struct {
	Status  int      `json:"-"`
	Code    string   `json:"errorCode"`
	Message string   `json:"message"`
	Fields  []string `json:"fields,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }
func Problem(status int, code, message string) error {
	return &APIError{Status: status, Code: code, Message: message}
}
func Invalid(code, message string) error { return Problem(http.StatusBadRequest, code, message) }

func FieldProblem(code, message, field string) error {
	return &APIError{Status: 400, Code: code, Message: message, Fields: []string{field}}
}
