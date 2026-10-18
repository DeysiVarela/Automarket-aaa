// Package web genera las páginas HTML en el servidor (html/template) y atiende sus formularios.
package web

import (
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

func NuevoRouter(h *Handlers, archivos fs.FS) (*gin.Engine, error) {
	plantillas, err := template.New("").Funcs(funciones).ParseFS(archivos, "web/templates/*.html")
	if err != nil {
		return nil, err
	}
	estaticos, err := fs.Sub(archivos, "web/static")
	if err != nil {
		return nil, err
	}
	sitio, err := fs.Sub(archivos, "web")
	if err != nil {
		return nil, err
	}
	spa, err := fs.ReadFile(sitio, "index.html")
	if err != nil {
		return nil, err
	}

	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil { // el navegador se conecta directo
		return nil, err
	}
	r.Use(gin.Logger(), gin.Recovery())
	r.SetHTMLTemplate(plantillas)
	r.StaticFS("/static", http.FS(estaticos))
	r.StaticFS("/spa/static", http.FS(estaticos))
	r.GET("/spa", func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, "/spa/")
	})
	r.GET("/spa/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", spa)
	})
	r.GET("/fotos/:nombre", h.Foto) // imágenes de vehículos, servidas desde el backend

	paginas := r.Group("", h.CargarUsuario)
	paginas.GET("/", h.Inicio)
	paginas.GET("/vehiculos/:id", h.DetalleVehiculo)
	paginas.GET("/ingresar", h.FormIngresar)
	paginas.POST("/ingresar", h.Ingresar)
	paginas.GET("/registro", h.FormRegistro)
	paginas.POST("/registro", h.Registrar)
	paginas.POST("/salir", h.Salir)

	panel := paginas.Group("/panel", RequiereSesion)
	panel.GET("", h.Panel)
	panel.POST("/vehiculos", h.Publicar)
	panel.POST("/borrador/fotos", h.CargarFotos)
	panel.GET("/borrador/fotos/:id", h.FotoBorrador)
	panel.POST("/borrador/fotos/:id/quitar", h.QuitarFoto)
	panel.POST("/vehiculos/:id/vendido", h.Accion(h.api.MarcarVendido, "El vehículo se marcó como vendido."))
	panel.GET("/admin/vehiculos/:id", h.DetalleAdmin)
	panel.POST("/admin/vehiculos/:id/aprobar", h.Accion(h.api.Aprobar, "Publicación aprobada y disponible en el catálogo."))
	panel.POST("/admin/vehiculos/:id/eliminar", h.Accion(h.api.EliminarVehiculo, "Publicación eliminada."))
	panel.POST("/admin/usuarios/:id/eliminar", h.Accion(h.api.EliminarUsuario, "Cuenta y publicaciones eliminadas."))

	return r, nil
}
