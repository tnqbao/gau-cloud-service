package middlewares

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tnqbao/gau-cloud-service/config"
	"github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/repository"
	"github.com/tnqbao/gau-cloud-service/utils"
)

const (
	// TimestampTolerance is the maximum allowed time difference in seconds
	TimestampTolerance = 300
)

// UploadAuthMiddleware creates a middleware that supports dual authentication:
// 1. Bearer JWT authentication (existing flow)
// 2. HMAC signature authentication (new flow for programmatic access)
//
// Authentication is OR logic - either method is acceptable.
func UploadAuthMiddleware(
	authService *infra.AuthorizationService,
	iamRepo *repository.IAMUserRepository,
	cfg *config.EnvConfig,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Check auth type
		if strings.HasPrefix(authHeader, "Bearer ") {
			// JWT authentication flow
			handleJWTAuth(c, authService, cfg, authHeader)
		} else if strings.HasPrefix(authHeader, "HMAC ") {
			// HMAC signature authentication flow
			handleHMACAuth(c, iamRepo, authHeader)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization type. Use 'Bearer' or 'HMAC'"})
			c.Abort()
			return
		}
	}
}

// handleJWTAuth processes Bearer JWT authentication
func handleJWTAuth(c *gin.Context, authService *infra.AuthorizationService, cfg *config.EnvConfig, authHeader string) {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Bearer token"})
		c.Abort()
		return
	}

	// Verify token with authorization service
	if err := authService.CheckAccessToken(tokenStr); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		c.Abort()
		return
	}

	// Parse token to extract claims
	parsedToken, err := utils.ParseToken(tokenStr, cfg)
	if err != nil || !parsedToken.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		c.Abort()
		return
	}

	// Inject claims to context
	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		if err := utils.InjectClaimsToContext(c, claims); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid claims"})
			c.Abort()
			return
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		c.Abort()
		return
	}

	c.Next()
}

// handleHMACAuth processes HMAC signature authentication
// Header format: Authorization: HMAC <accessKey>:<signature>
// Required headers: X-Timestamp
func handleHMACAuth(c *gin.Context, iamRepo *repository.IAMUserRepository, authHeader string) {
	// Parse HMAC header: HMAC <accessKey>:<signature>
	hmacValue := strings.TrimPrefix(authHeader, "HMAC ")
	parts := strings.SplitN(hmacValue, ":", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid HMAC authorization format. Expected: HMAC <accessKey>:<signature>"})
		c.Abort()
		return
	}

	accessKey := parts[0]
	clientSignature := parts[1]

	if accessKey == "" || clientSignature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Access key and signature are required"})
		c.Abort()
		return
	}

	// Validate X-Timestamp header
	timestampStr := c.GetHeader("X-Timestamp")
	if timestampStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Timestamp header is required"})
		c.Abort()
		return
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid X-Timestamp format"})
		c.Abort()
		return
	}

	// Anti-replay protection: check timestamp is within tolerance
	serverTime := time.Now().Unix()
	if utils.Abs(serverTime-timestamp) > TimestampTolerance {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Request timestamp expired"})
		c.Abort()
		return
	}

	// Load IAM user by access key
	iamUser, err := iamRepo.GetByAccessKey(accessKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access key"})
		c.Abort()
		return
	}

	// Read request body for hashing
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			c.Abort()
			return
		}
		// Restore body for subsequent handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Compute body hash
	bodyHash := utils.HashBodySHA256(bodyBytes)

	// Build string_to_sign: METHOD\nPATH\nTIMESTAMP\nSHA256(body)
	stringToSign := utils.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		timestamp,
		bodyHash,
	)

	// Compute server signature
	serverSignature := utils.ComputeHMACSHA256(iamUser.SecretKey, stringToSign)

	// Constant-time comparison to prevent timing attacks
	if !utils.SecureCompare(serverSignature, clientSignature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		c.Abort()
		return
	}

	// Authentication successful - inject user info to context
	c.Set("user_id", iamUser.UserId.String())
	c.Set("iam_user_id", iamUser.ID.String())
	c.Set("auth_method", "hmac")

	c.Next()
}
