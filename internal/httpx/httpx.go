// Package httpx holds small HTTP helpers: JSON writing and middleware.
package httpx

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

// JSON writes v as a JSON body with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // match Express/JSON.stringify output

	if err := enc.Encode(v); err != nil {
		log.Printf("[JSON] encode error: %v", err)
		http.Error(w, `{"success":false,"message":"Failed to encode response."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// ErrorBody is the {success:false, message, error} envelope used by the
// controllers. Field order matches the Node responses.
type ErrorBody struct {
	Success bool    `json:"success" example:"false"`
	Message string  `json:"message" example:"Customer not found."`
	Error   *string `json:"error,omitempty" extensions:"x-nullable"`
}

// Error writes an ErrorBody with the given status code.
func Error(w http.ResponseWriter, status int, message string, err error) {
	body := ErrorBody{Success: false, Message: message}
	if err != nil {
		msg := err.Error()
		body.Error = &msg
	}
	JSON(w, status, body)
}

// CORS mirrors the permissive `cors()` default used by src/app.js.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Recover is the equivalent of the global error handler in src/app.js.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[Global Error] %v", rec)
				JSON(w, http.StatusInternalServerError, ErrorBody{
					Success: false,
					Message: "Internal server error.",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// NotFound matches the 404 body produced by src/app.js.
func NotFound(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusNotFound, ErrorBody{
		Success: false,
		Message: "Route " + r.Method + " " + r.URL.Path + " not found.",
	})
}
