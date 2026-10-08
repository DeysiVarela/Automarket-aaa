package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	auditKey := []byte(os.Getenv("AUDIT_KEY"))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatal("PORT debe ser un número entre 1 y 65535")
	}
	if len(auditKey) < 32 {
		log.Fatal("AUDIT_KEY debe tener al menos 32 bytes")
	}
	if len(adminPassword) < minimumPasswordLength {
		log.Fatal("ADMIN_PASSWORD debe tener al menos 12 bytes")
	}

	store := newStore()
	admin, err := store.createInitialAdmin(adminEmail, adminPassword)
	if err != nil {
		log.Fatalf("No se pudo crear el administrador inicial: %v", err)
	}
	log.Printf("Administrador inicial configurado: %s", admin.Email)

	audit, err := openAuditLog(os.Getenv("AUDIT_LOG_PATH"), auditKey)
	if err != nil {
		log.Fatalf("No se pudo iniciar la bitácora de auditoría: %v", err)
	}
	defer audit.Close()

	router := gin.New()
	router.Use(audit.middleware(), gin.Recovery())
	registerRoutes(router, store)

	log.Printf("AutoMarket API escuchando en :%d (HTTP)", portNumber)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("El servidor terminó con error: %v", err)
	}
}

func registerRoutes(router *gin.Engine, store *store) {
	router.POST("/api/users", registerHandler(store))
	router.POST("/api/sessions", loginHandler(store))

	router.GET("/api/vehicles", listPublicVehiclesHandler(store))
	router.GET("/api/vehicles/:id", getPublicVehicleHandler(store))

	sellers := router.Group("/api")
	sellers.Use(requireRole(store, roleSeller))
	sellers.POST("/vehicles", createVehicleHandler(store))
	sellers.POST("/vehicles/:id/sold", reportSoldHandler(store))

	admins := router.Group("/api/admin")
	admins.Use(requireRole(store, roleAdmin))
	admins.GET("/vehicles/pending", listPendingVehiclesHandler(store))
	admins.POST("/vehicles/:id/approve", approveVehicleHandler(store))
	admins.DELETE("/vehicles/:id", deleteVehicleHandler(store))
	admins.DELETE("/users/:id", deleteUserHandler(store))
}
