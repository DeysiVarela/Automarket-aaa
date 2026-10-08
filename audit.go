package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type auditPayload struct {
	Timestamp  string `json:"timestamp"`
	ActorID    string `json:"actor_id"`
	ActorRole  string `json:"actor_role"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	ResourceID string `json:"resource_id,omitempty"`
	Status     int    `json:"status"`
	RemoteAddr string `json:"remote_addr"`
	Previous   string `json:"previous_hash"`
}

type auditRecord struct {
	auditPayload
	Hash string `json:"hash"`
}

type auditLog struct {
	mu       sync.Mutex
	file     *os.File
	key      []byte
	lastHash string
}

func openAuditLog(path string, key []byte) (*auditLog, error) {
	if path == "" {
		path = "audit.log"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	result := &auditLog{file: file, key: append([]byte(nil), key...)}
	if err := result.verifyExisting(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return result, nil
}

func (a *auditLog) verifyExisting() error {
	if _, err := a.file.Seek(0, 0); err != nil {
		return err
	}
	scanner := bufio.NewScanner(a.file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	expectedPrevious := ""
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := append([]byte(nil), scanner.Bytes()...)
		var record auditRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return fmt.Errorf("bitácora inválida en la línea %d: %w", lineNumber, err)
		}
		if record.Previous != expectedPrevious {
			return fmt.Errorf("cadena de auditoría rota en la línea %d", lineNumber)
		}
		payload, err := json.Marshal(record.auditPayload)
		if err != nil {
			return err
		}
		expectedHash := a.hash(payload)
		if !hmac.Equal([]byte(record.Hash), []byte(expectedHash)) {
			return fmt.Errorf("integridad de auditoría no válida en la línea %d", lineNumber)
		}
		expectedPrevious = record.Hash
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	a.lastHash = expectedPrevious
	_, err := a.file.Seek(0, 2)
	return err
}

func (a *auditLog) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		actorID, _ := c.Get("actor_id")
		actorRole, _ := c.Get("actor_role")
		resourceID, _ := c.Get("resource_id")
		payload := auditPayload{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			ActorID:    stringValue(actorID, "anonymous"),
			ActorRole:  stringValue(actorRole, "anonymous"),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			ResourceID: stringValue(resourceID, ""),
			Status:     c.Writer.Status(),
			RemoteAddr: c.Request.RemoteAddr,
		}
		if err := a.append(payload); err != nil {
			log.Printf("ERROR: no se pudo guardar la auditoría de %s %s: %v", payload.Method, payload.Path, err)
		}
	}
}

func (a *auditLog) append(payload auditPayload) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	payload.Previous = a.lastHash
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record := auditRecord{auditPayload: payload, Hash: a.hash(encodedPayload)}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	n, err := a.file.Write(line)
	if err != nil {
		return err
	}
	if n != len(line) {
		return errors.New("escritura incompleta de la bitácora")
	}
	if err := a.file.Sync(); err != nil {
		return err
	}
	a.lastHash = record.Hash
	return nil
}

func (a *auditLog) hash(payload []byte) string {
	mac := hmac.New(sha256.New, a.key)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *auditLog) Close() error {
	return a.file.Close()
}

func stringValue(value any, fallback string) string {
	result, ok := value.(string)
	if !ok || strings.TrimSpace(result) == "" {
		return fallback
	}
	return result
}
