package middleware

import (
	"net/http"
	"strings"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const claimsKey = "auth_claims"

// JWTMiddleware validates bearer tokens on protected routes.
func JWTMiddleware(secret string) gin.HandlerFunc {
	v := auth.NewValidator(auth.ConfigFromSecret(secret))
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := v.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

// APIKeyMiddleware validates X-API-Key header as an alternative to jwt.
func APIKeyMiddleware(header string, validKeys []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(validKeys))
	for _, k := range validKeys {
		allowed[k] = struct{}{}
	}
	return func(c *gin.Context) {
		key := c.GetHeader(header)
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}
		if _, ok := allowed[key]; !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Set(claimsKey, jwt.MapClaims{"sub": "api-key"})
		c.Next()
	}
}

// AuthMiddleware accepts either jwt or api key — whichever is present.
func AuthMiddleware(secret string, header string, validKeys []string) gin.HandlerFunc {
	jwtMW := JWTMiddleware(secret)
	keyMW := APIKeyMiddleware(header, validKeys)
	return func(c *gin.Context) {
		if strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
			jwtMW(c)
			return
		}
		if c.GetHeader(header) != "" {
			keyMW(c)
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	}
}

func extractBearer(h string) string {
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
