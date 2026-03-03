package logrequest

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
    cookieName   = "auth"          // имя cookie
    secretKey    = "my-super-secret-key" // в реальном проекте брать из env
    cookieMaxAge = 30 * 24 * time.Hour // 30 дней
)

type contextKey string

const (
    UserIDKey contextKey = "userID"
)

// signData создаёт HMAC-SHA256 подпись для данных
func signData(data string) string {
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}

// validateCookie проверяет cookie и возвращает userID, если подпись верна
func validateCookie(cookieValue string) (string, bool) {
    parts := strings.SplitN(cookieValue, ":", 2)
    if len(parts) != 2 {
        return "", false
    }
    userID, signature := parts[0], parts[1]
    expectedSign := signData(userID)
    if !hmac.Equal([]byte(signature), []byte(expectedSign)) {
        return "", false
    }
    return userID, true
}

// generateUserID создаёт новый UUID v4 (16 random bytes, hex-encoded as 32 chars + hyphens)
func generateUserID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// setAuthCookie создаёт и устанавливает cookie с подписанным userID
func setAuthCookie(w http.ResponseWriter, userID string) {
    signature := signData(userID)
    cookieValue := userID + ":" + signature
    http.SetCookie(w, &http.Cookie{
        Name:     cookieName,
        Value:    cookieValue,
        Path:     "/",
        MaxAge:   int(cookieMaxAge.Seconds()),
        HttpOnly: true,
        Secure:   false, // в продакшене должно быть true (HTTPS)
        SameSite: http.SameSiteStrictMode,
    })
}




// AuthMiddleware проверяет/устанавливает cookie и добавляет userID в контекст
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var userID string
        var needSetCookie bool

        // Пытаемся получить cookie
        cookie, err := r.Cookie(cookieName)
        if err == nil {
            // Если cookie есть, проверяем подпись
            if uid, ok := validateCookie(cookie.Value); ok {
                userID = uid
                needSetCookie = false
            } else {
                // Подпись не совпадает — генерируем новый ID
                userID = generateUserID()
                needSetCookie = true
            }
        } else {
            // Cookie отсутствует — создаём нового пользователя
            userID = generateUserID()
            needSetCookie = true
        }

        // Если нужно, устанавливаем новую cookie
        if needSetCookie {
            setAuthCookie(w, userID)
        }

        // Кладём userID в контекст
        ctx := context.WithValue(r.Context(), UserIDKey, userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

