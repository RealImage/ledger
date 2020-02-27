package handler

import (
	"net/http"

	"github.com/RealImage/QLedger/utils"
)

// Ping responds 200 OK when the server is up and healthy
func Ping(w http.ResponseWriter, r *http.Request) {
	// TODO: Should DB connection check be made while ping ?
	pong := struct {
		Ping string `json:"ping"`
	}{
		Ping: "pong",
	}
	utils.WriteResponse(w, &pong, http.StatusOK)
}
