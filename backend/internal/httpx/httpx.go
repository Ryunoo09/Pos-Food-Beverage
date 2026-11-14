// Package httpx menyediakan format respons API dan error bisnis yang konsisten.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// AppError adalah error bisnis dengan kode string, pesan Bahasa Indonesia,
// dan status HTTP yang dikirim ke klien.
type AppError struct {
	Code       string
	Message    string
	Status     int
	Details    any
	RetryAfter int
}

func (e *AppError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func NewError(status int, code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: status}
}

func Errorf(status int, code, message string, details any) *AppError {
	return &AppError{Code: code, Message: message, Status: status, Details: details}
}

// WriteData mengirim {data, meta.request_id}.
func WriteData(w http.ResponseWriter, status int, data any, requestID string) {
	writeJSON(w, status, map[string]any{
		"data": data,
		"meta": map[string]string{"request_id": requestID},
	})
}

// WritePage mengirim list dengan metadata paginasi.
func WritePage(w http.ResponseWriter, data any, meta PageMeta) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"meta": meta,
	})
}

// PageMeta adalah metadata paginasi pada envelope respons.
type PageMeta struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	Total      int    `json:"total"`
	TotalPages int    `json:"total_pages"`
	RequestID  string `json:"request_id"`
}

// WriteError memetakan AppError ke envelope error; error lain menjadi 500
// tanpa membocorkan detail internal ke klien.
func WriteError(w http.ResponseWriter, err error, requestID string) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		slog.Error("unhandled error", "error", err, "request_id", requestID)
		appErr = NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan internal.")
	}

	if appErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprint(appErr.RetryAfter))
	}

	payload := map[string]any{
		"error": map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
		},
		"meta": map[string]string{"request_id": requestID},
	}
	if appErr.Details != nil {
		payload["error"].(map[string]any)["details"] = appErr.Details
	}
	writeJSON(w, appErr.Status, payload)
}

// DecodeJSON membaca body JSON dengan batas 1 MB dan menolak field tak dikenal.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return NewError(http.StatusBadRequest, "VALIDATION_ERROR", "Body permintaan tidak valid.")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write response", "error", err)
	}
}
