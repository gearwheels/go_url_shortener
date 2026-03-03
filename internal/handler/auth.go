package handler

import (
	"log/slog"
	"net/http"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
    jwt.RegisteredClaims
    UserID int
}

func AuthUser(w http.ResponseWriter, r *http.Request) {

	// r.Cookie()
	http.SetCookie(w, )

	

	w.WriteHeader(http.StatusOK)
	slog.Info("Data base alive!")
	return
	
}
