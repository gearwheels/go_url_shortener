package logrequest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mw_test")
	if err != nil {
		panic(err)
	}
	config.Init("localhost:8080", "http://localhost:8080/",
		filepath.Join(dir, "store.txt"), "", "test-secret-key", "", "")
	os.Exit(m.Run())
}

func TestContextWithUserID_AndGetUserID(t *testing.T) {
	ctx := ContextWithUserID(t.Context(), "user-42")

	uid, err := GetUserID(ctx)
	if err != nil {
		t.Fatalf("GetUserID: unexpected error: %v", err)
	}
	if uid != "user-42" {
		t.Errorf("expected user-42, got %q", uid)
	}
}

func TestGetUserID_NotInContext(t *testing.T) {
	_, err := GetUserID(t.Context())
	if err == nil {
		t.Fatal("expected error when userID not in context")
	}
}

func TestGetUserID_EmptyString(t *testing.T) {
	ctx := ContextWithUserID(t.Context(), "")
	_, err := GetUserID(ctx)
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

func TestAuthMiddleware_NoCookie(t *testing.T) {
	var capturedUID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := GetUserID(r.Context())
		if err != nil {
			t.Errorf("GetUserID in handler: %v", err)
		}
		capturedUID = uid
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)

	if capturedUID == "" {
		t.Error("expected non-empty userID to be generated")
	}
	// Should set a new auth cookie
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth" {
			found = true
		}
	}
	if !found {
		t.Error("expected auth cookie to be set")
	}
}

func TestAuthMiddleware_ValidCookie(t *testing.T) {
	userID := "existing-user"
	sig := signData(userID)
	cookieVal := userID + ":" + sig

	var capturedUID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := GetUserID(r.Context())
		capturedUID = uid
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: cookieVal})
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)

	if capturedUID != userID {
		t.Errorf("expected %q, got %q", userID, capturedUID)
	}
}

func TestAuthMiddleware_InvalidCookieSignature(t *testing.T) {
	cookieVal := "some-user:badsignature"

	var capturedUID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUID, _ = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: cookieVal})
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)

	// Should generate a new userID, not use the tampered one
	if capturedUID == "some-user" {
		t.Error("expected a new userID, not the tampered one")
	}
	if capturedUID == "" {
		t.Error("expected a newly generated non-empty userID")
	}
}

func TestAuthMiddleware_MalformedCookie(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := GetUserID(r.Context())
		if err != nil || uid == "" {
			t.Errorf("expected valid userID in context, err=%v uid=%q", err, uid)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: "no-colon-here"})
	rr := httptest.NewRecorder()
	AuthMiddleware(next).ServeHTTP(rr, req)

	_ = rr
}

func TestValidateCookie_Valid(t *testing.T) {
	uid := "test-uid"
	sig := signData(uid)
	got, err := validateCookie(uid + ":" + sig)
	if err != nil {
		t.Fatalf("validateCookie: %v", err)
	}
	if got != uid {
		t.Errorf("expected %q, got %q", uid, got)
	}
}

func TestValidateCookie_InvalidFormat(t *testing.T) {
	_, err := validateCookie("nocoion")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestValidateCookie_InvalidSignature(t *testing.T) {
	_, err := validateCookie("uid:wrongsig")
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestGenerateUserID_NonEmpty(t *testing.T) {
	id := generateUserID()
	if id == "" {
		t.Error("generateUserID returned empty string")
	}
	// Should look like a UUID (contains dashes) or a numeric timestamp fallback
	if !strings.Contains(id, "-") && len(id) < 5 {
		t.Errorf("unexpected generateUserID format: %q", id)
	}
}
