package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// EmptyBodyHash is the SHA256 hash of an empty body
const EmptyBodyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// BuildStringToSign constructs the canonical string to sign for HMAC authentication.
// Format: METHOD\nPATH\nTIMESTAMP\nSHA256(body)
//
// Parameters:
//   - method: HTTP method in uppercase (POST, PUT, etc.)
//   - path: URL path without domain and query string
//   - timestamp: Unix timestamp in seconds
//   - bodyHash: SHA256 hex hash of the request body (use EmptyBodyHash for empty body)
//
// Returns the canonical string to be signed
func BuildStringToSign(method, path string, timestamp int64, bodyHash string) string {
	return fmt.Sprintf("%s\n%s\n%d\n%s", method, path, timestamp, bodyHash)
}

// ComputeHMACSHA256 computes HMAC-SHA256 signature and returns hex-encoded string.
//
// Parameters:
//   - secretKey: The secret key for HMAC computation
//   - message: The message to sign (typically the string_to_sign)
//
// Returns hex-encoded signature (64 characters)
func ComputeHMACSHA256(secretKey, message string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// SecureCompare performs constant-time string comparison to prevent timing attacks.
// This MUST be used when comparing signatures.
//
// Returns true if both strings are equal, false otherwise.
func SecureCompare(a, b string) bool {
	// Convert to bytes for constant-time comparison
	aBytes := []byte(a)
	bBytes := []byte(b)

	// subtle.ConstantTimeCompare returns 1 if equal, 0 otherwise
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}

// HashBodySHA256 computes SHA256 hash of body bytes and returns hex-encoded string.
// If body is nil or empty, returns EmptyBodyHash constant.
//
// Returns hex-encoded hash (64 characters)
func HashBodySHA256(body []byte) string {
	if len(body) == 0 {
		return EmptyBodyHash
	}
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// Abs returns the absolute value of x
func Abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
