package health

import (
	"encoding/json"
	"net/http"
)

type Handler struct{}

type Response struct {
	Status string `json:"status"`
}

func NewHandler() Handler {
	return Handler{}
}

func (Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(Response{Status: "ok"})
}
