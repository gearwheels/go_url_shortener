package logrequest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
    cookieName   = "auth"          // имя cookie
    secretKey    = "my-super-secret-key" // в реальном проекте брать из env
    cookieMaxAge = 30 * 24 * time.Hour // 30 дней
)

type contextKey string

const (
	userIDKey contextKey = "userID"
)

// GetUserID извлекает userID из контекста. Возвращает ("", false), если значение
// отсутствует или имеет неожиданный тип. Возвращает (userID, true) для корректного string.
func GetUserID(ctx context.Context) (string, bool) {
	v := ctx.Value(userIDKey)
	if v == nil {
		return "", false
	}
	userID, ok := v.(string)
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

// signData создаёт HMAC-SHA256 подпись для данных
func signData(data string) string {
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}

// Ошибки валидации cookie (для проверки через errors.Is в будущем).
var (
	ErrInvalidCookieFormat = errors.New("cookie has invalid format: expected \"userID:signature\"")
	ErrInvalidSignature    = errors.New("cookie signature is invalid")
)

// validateCookie проверяет cookie и возвращает userID при верной подписи.
func validateCookie(cookieValue string) (string, error) {
	parts := strings.SplitN(cookieValue, ":", 2)
	if len(parts) != 2 {
		return "", ErrInvalidCookieFormat
	}
	userID, signature := parts[0], parts[1]
	expectedSign := signData(userID)
	if !hmac.Equal([]byte(signature), []byte(expectedSign)) {
		return "", ErrInvalidSignature
	}
	return userID, nil
}

// generateUserID создаёт новый UUID v7 (время-упорядоченный, RFC 9562).
func generateUserID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return id.String()
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
            uid, errValidate := validateCookie(cookie.Value)
            if errValidate == nil {
                userID = uid
                needSetCookie = false
            } else {
                // Неверный формат или подпись — генерируем новый ID
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
        ctx := context.WithValue(r.Context(), userIDKey, userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

