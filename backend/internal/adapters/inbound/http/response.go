package http

import (
	"encoding/json"
	"net/http"
	"time"
)

// ErrorDetail describes an issue with a specific input field.
type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// ErrorResponse matches the OpenAPI ErrorResponse schema.
type ErrorResponse struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Details   []ErrorDetail `json:"details,omitempty"`
	RequestID string        `json:"request_id"`
	Timestamp string        `json:"timestamp"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, code, message, requestID string, details ...ErrorDetail) {
	resp := ErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(w, status, resp)
}
