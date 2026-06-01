package handlers

import (
	"net/http"

	"github.com/nazscentsation/shop/internal/utils"
)

func Health(w http.ResponseWriter, r *http.Request) {
	utils.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
