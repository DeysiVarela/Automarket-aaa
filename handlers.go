package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func registerHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request registerRequest
		if !decodeJSON(c, &request) {
			return
		}
		account, err := accounts.register(request.Name, request.Email, request.Phone, request.Password)
		if err != nil {
			if errors.Is(err, errEmailInUse) {
				writeError(c, http.StatusConflict, "email is already registered")
				return
			}
			var invalid validationError
			if errors.As(err, &invalid) {
				writeError(c, http.StatusBadRequest, invalid.Error())
				return
			}
			writeError(c, http.StatusInternalServerError, "could not register user")
			return
		}
		c.Set("resource_id", account.ID)
		c.JSON(http.StatusCreated, account)
	}
}

func loginHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request loginRequest
		if !decodeJSON(c, &request) {
			return
		}
		account, token, err := accounts.login(request.Email, request.Password)
		if err != nil {
			if errors.Is(err, errInvalidLogin) {
				writeError(c, http.StatusUnauthorized, "invalid email or password")
				return
			}
			writeError(c, http.StatusInternalServerError, "could not create session")
			return
		}
		c.Set("actor_id", account.ID)
		c.Set("actor_role", string(account.Role))
		c.Set("resource_id", account.ID)
		c.JSON(http.StatusOK, gin.H{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_at":   time.Now().UTC().Add(sessionDuration),
			"user":         account,
		})
	}
}

func currentUserHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		account, exists := c.Get("actor_user")
		if !exists {
			writeError(c, http.StatusUnauthorized, "authentication required")
			return
		}
		c.JSON(http.StatusOK, account)
	}
}

func listSellerVehiclesHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _ := c.Get("actor_id")
		c.JSON(http.StatusOK, gin.H{"vehicles": accounts.sellerVehicles(actorID.(string))})
	}
}

func listUsersHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"users": accounts.listUsers()})
	}
}

func listPublicVehiclesHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"vehicles": accounts.publicVehicles()})
	}
}

func getPublicVehicleHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		item, ok := accounts.publicVehicle(c.Param("id"))
		if !ok {
			writeError(c, http.StatusNotFound, "vehicle not found")
			return
		}
		c.Set("resource_id", item.ID)
		c.JSON(http.StatusOK, item)
	}
}

func createVehicleHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input vehicleInput
		if !decodeJSON(c, &input) {
			return
		}
		actorID, _ := c.Get("actor_id")
		item, err := accounts.createVehicle(actorID.(string), input)
		if err != nil {
			if errors.Is(err, errForbidden) {
				writeError(c, http.StatusForbidden, "operation not permitted")
				return
			}
			var invalid validationError
			if errors.As(err, &invalid) {
				writeError(c, http.StatusBadRequest, invalid.Error())
				return
			}
			writeError(c, http.StatusInternalServerError, "could not create vehicle")
			return
		}
		c.Set("resource_id", item.ID)
		c.JSON(http.StatusCreated, item)
	}
}

func reportSoldHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _ := c.Get("actor_id")
		id := c.Param("id")
		c.Set("resource_id", id)
		if err := accounts.reportSold(actorID.(string), id); err != nil {
			writeStoreError(c, err, "could not report vehicle as sold")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listPendingVehiclesHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"vehicles": accounts.pendingVehicles()})
	}
}

func listAdminVehiclesHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"vehicles": accounts.allVehicles()})
	}
}

func approveVehicleHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _ := c.Get("actor_id")
		id := c.Param("id")
		c.Set("resource_id", id)
		if err := accounts.approveVehicle(actorID.(string), id); err != nil {
			writeStoreError(c, err, "could not approve vehicle")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func deleteVehicleHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _ := c.Get("actor_id")
		id := c.Param("id")
		c.Set("resource_id", id)
		if err := accounts.deleteVehicle(actorID.(string), id); err != nil {
			writeStoreError(c, err, "could not delete vehicle")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func deleteUserHandler(accounts *store) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _ := c.Get("actor_id")
		id := c.Param("id")
		c.Set("resource_id", id)
		if err := accounts.deleteUser(actorID.(string), id); err != nil {
			writeStoreError(c, err, "could not delete user")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func decodeJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(c, http.StatusBadRequest, "request body must be valid JSON with only supported fields")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(c, http.StatusBadRequest, "request body must contain exactly one JSON value")
		return false
	}
	return true
}

func writeStoreError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, errNotFound):
		writeError(c, http.StatusNotFound, "resource not found")
	case errors.Is(err, errForbidden):
		writeError(c, http.StatusForbidden, "operation not permitted")
	case errors.Is(err, errInvalidState):
		writeError(c, http.StatusConflict, "resource is not in a valid state for this operation")
	case errors.Is(err, errCannotDeleteSelf), errors.Is(err, errNoAdminRemains):
		writeError(c, http.StatusConflict, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallback)
	}
}

func writeError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
