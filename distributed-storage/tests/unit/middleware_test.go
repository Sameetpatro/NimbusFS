package unit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func ginTestContext(w http.ResponseWriter, r *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	return c
}

func TestAuthMiddlewareRejectsMissingKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := ginTestContext(w, r)

	middleware.AuthMiddleware("secret", "X-API-Key", []string{"k"})(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsAPIKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "k")
	w := httptest.NewRecorder()
	c := ginTestContext(w, r)

	middleware.AuthMiddleware("secret", "X-API-Key", []string{"k"})(c)
	if c.IsAborted() {
		t.Fatal("should not abort valid key")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	mw := middleware.NewRateLimitMiddleware(2, 2)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		c := ginTestContext(w, r)
		mw(c)
		if i < 2 && w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d should pass", i)
		}
	}
}
