// Front de AutoMarket: Go + Gin. Las páginas se generan en el servidor y los datos
// se obtienen de la API del backend; el navegador solo recibe HTML y CSS.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/DeysiVarela/Automarket-aaa/internal/cliente"
	"github.com/DeysiVarela/Automarket-aaa/internal/web"
)

//go:embed web
var archivos embed.FS

func main() {
	api := cliente.Nueva(env("BACK_URL", "http://localhost:8080"))
	router, err := web.NuevoRouter(web.NuevosHandlers(api), archivos)
	if err != nil {
		log.Fatalf("no se pudo iniciar el front: %v", err)
	}

	direccion := ":" + env("PUERTO", "3000")
	log.Printf("AutoMarket front en http://localhost%s", direccion)
	if err := router.Run(direccion); err != nil {
		log.Fatal(err)
	}
}

func env(clave, porDefecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return porDefecto
}
