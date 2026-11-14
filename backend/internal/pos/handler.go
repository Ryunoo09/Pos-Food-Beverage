package pos

import (
	"net/http"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

// PreviewTotal menghitung total order tanpa menyimpan apa pun. Dipakai POS untuk
// menampilkan angka otoritatif sebelum konfirmasi.
func (h *Handler) PreviewTotal(w http.ResponseWriter, r *http.Request) {
	requestID := httpx.RequestIDFrom(r.Context())

	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	result, err := Calculate(in)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}
	httpx.WriteData(w, http.StatusOK, result, requestID)
}
