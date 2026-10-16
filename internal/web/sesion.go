package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/DeysiVarela/Automarket-aaa/internal/cliente"
)

const (
	cookieSesion = "automarket_sesion"
	cookieAviso  = "automarket_aviso"
	duracionSeg  = 8 * 60 * 60 // igual que la vigencia del token en el backend
	claveUsuario = "usuario"
)

// fijarCookie crea cookies HttpOnly y SameSite=Lax: el navegador no las expone a
// scripts ni las envía en formularios de otros sitios. Sin Secure porque no hay TLS.
func fijarCookie(c *gin.Context, nombre, valor string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(nombre, valor, maxAge, "/", "", false, true)
}

func token(c *gin.Context) string {
	t, _ := c.Cookie(cookieSesion)
	return t
}

// peticion arma los datos con los que el front llama al backend en nombre del navegador.
func peticion(c *gin.Context) cliente.Peticion {
	return cliente.Peticion{Ctx: c.Request.Context(), Token: token(c), IP: c.ClientIP()}
}

// CargarUsuario resuelve la cookie de sesión contra el backend (GET /api/me).
// Si el token ya no es válido, borra la cookie.
func (h *Handlers) CargarUsuario(c *gin.Context) {
	if token(c) != "" {
		u, err := h.api.UsuarioActual(peticion(c))
		switch {
		case err == nil:
			c.Set(claveUsuario, u)
		case esNoAutenticado(err):
			fijarCookie(c, cookieSesion, "", -1)
		}
	}
	c.Next()
}

// RequiereSesion redirige al ingreso si no hay un usuario autenticado.
func RequiereSesion(c *gin.Context) {
	if _, ok := c.Get(claveUsuario); !ok {
		c.Redirect(http.StatusSeeOther, "/ingresar")
		c.Abort()
		return
	}
	c.Next()
}

func usuarioActual(c *gin.Context) *cliente.Usuario {
	if u, ok := c.Get(claveUsuario); ok {
		usuario := u.(cliente.Usuario)
		return &usuario
	}
	return nil
}

func esNoAutenticado(err error) bool {
	var e *cliente.ErrorAPI
	return errors.As(err, &e) && e.Estado == http.StatusUnauthorized
}

// ---------- Avisos de una sola lectura (patrón POST → redirect → GET) ----------

type aviso struct {
	Error   bool
	Mensaje string
}

func avisar(c *gin.Context, esError bool, mensaje string) {
	tipo := "ok"
	if esError {
		tipo = "error"
	}
	fijarCookie(c, cookieAviso, url.QueryEscape(tipo+"|"+mensaje), 60)
}

func leerAviso(c *gin.Context) *aviso {
	valor, err := c.Cookie(cookieAviso)
	if err != nil {
		return nil
	}
	fijarCookie(c, cookieAviso, "", -1)
	texto, err := url.QueryUnescape(valor)
	if err != nil {
		return nil
	}
	tipo, mensaje, _ := strings.Cut(texto, "|")
	return &aviso{Error: tipo == "error", Mensaje: mensaje}
}
