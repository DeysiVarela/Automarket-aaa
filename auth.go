package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	minimumPasswordLength = 12
	passwordRounds        = 120000
)

func hashPassword(password string) ([]byte, []byte, error) {
	if len(password) < minimumPasswordLength || len(password) > 1024 {
		return nil, nil, validationError{"password must contain 12 to 1024 bytes"}
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, derivePasswordKey(password, salt), nil
}

func verifyPassword(password string, salt, expected []byte) bool {
	if len(password) > 1024 || len(expected) == 0 {
		return false
	}
	actual := derivePasswordKey(password, salt)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func derivePasswordKey(password string, salt []byte) []byte {
	mac := hmac.New(sha256.New, []byte(password))
	_, _ = mac.Write(salt)
	_, _ = mac.Write([]byte{0, 0, 0, 1})
	u := mac.Sum(nil)
	result := append([]byte(nil), u...)
	for i := 1; i < passwordRounds; i++ {
		mac = hmac.New(sha256.New, []byte(password))
		_, _ = mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}

func encodeToken(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func validateEmail(email string) error {
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return validationError{"email must be a valid address"}
	}
	return nil
}

func requireRole(accounts *store, allowed ...role) gin.HandlerFunc {
	allowedRoles := make(map[role]struct{}, len(allowed))
	for _, current := range allowed {
		allowedRoles[current] = struct{}{}
	}

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || len(header) <= len("Bearer ") {
			writeError(c, 401, "authentication required")
			return
		}
		account, ok := accounts.authenticate(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if !ok {
			writeError(c, 401, "invalid or expired credentials")
			return
		}
		c.Set("actor_id", account.ID)
		c.Set("actor_role", string(account.Role))
		if _, ok := allowedRoles[account.Role]; !ok {
			writeError(c, 403, "operation not permitted for this role")
			return
		}
		c.Next()
	}
}
