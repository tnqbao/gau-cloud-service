package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/config"

	"strings"
)

func ExtractToken(c *gin.Context) string {
	if token, err := c.Cookie("access_token"); err == nil && token != "" {
		return token
	}
	authHeader := c.GetHeader("Authorization")
	parts := strings.Fields(authHeader)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return ""
}

func ParseToken(tokenString string, config *config.EnvConfig) (*jwt.Token, error) {
	secret := []byte(config.JWT.SecretKey)
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
}

func InjectClaimsToContext(c *gin.Context, claims jwt.MapClaims) error {
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return errors.New("Invalid user_id format")
	}
	// Validate that it's a valid UUID
	_, err := uuid.Parse(userIDStr)
	if err != nil {
		return errors.New("Invalid user_id format")
	}
	// Set as string to support both string and UUID retrieval
	c.Set("user_id", userIDStr)

	if permission, ok := claims["permission"].(string); ok {
		c.Set("permission", permission)
	} else {
		c.Set("permission", "")
	}
	return nil
}

// It supports both string and uuid.UUID types and returns a parsed UUID
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID := c.MustGet("user_id")
	if userID == nil {
		return uuid.Nil, errors.New("user_id is missing from context")
	}

	var uuidUserID uuid.UUID
	switch v := userID.(type) {
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, errors.New("invalid user_id format: " + err.Error())
		}
		uuidUserID = parsed
	case uuid.UUID:
		uuidUserID = v
	default:
		return uuid.Nil, errors.New("invalid user_id type in context")
	}

	return uuidUserID, nil
}
