package web

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/DeysiVarela/Automarket-aaa/internal/cliente"
)

type Handlers struct {
	api        *cliente.API
	borradores *Borradores
}

func NuevosHandlers(api *cliente.API) *Handlers {
	return &Handlers{api: api, borradores: NuevosBorradores()}
}

// render agrega a toda vista el usuario de la sesión y el aviso pendiente
// (salvo que la vista ya traiga uno propio).
func render(c *gin.Context, estado int, vista string, datos gin.H) {
	if datos == nil {
		datos = gin.H{}
	}
	datos["Usuario"] = usuarioActual(c)
	pendiente := leerAviso(c)
	if _, ok := datos["Aviso"]; !ok {
		datos["Aviso"] = pendiente
	}
	c.HTML(estado, vista, datos)
}

func redirigir(c *gin.Context, ruta string) { c.Redirect(http.StatusSeeOther, ruta) }

// fallo muestra el error del backend en la página de destino; si la sesión venció, pide ingresar.
func fallo(c *gin.Context, err error, destino string) {
	if esNoAutenticado(err) {
		fijarCookie(c, cookieSesion, "", -1)
		destino = "/ingresar"
	}
	avisar(c, true, err.Error())
	redirigir(c, destino)
}

// ---------- HU-VIS-01: catálogo público ----------

func (h *Handlers) Inicio(c *gin.Context) {
	texto := strings.TrimSpace(c.Query("q"))
	anio, _ := strconv.Atoi(c.Query("anio"))
	datos := gin.H{"Q": texto, "Anio": c.Query("anio")}

	vehiculos, err := h.api.Catalogo(peticion(c))
	if err != nil {
		datos["Aviso"] = &aviso{Error: true, Mensaje: err.Error()}
	}
	datos["Vehiculos"] = filtrar(vehiculos, texto, anio)
	render(c, http.StatusOK, "inicio.html", datos)
}

// DetalleVehiculo es la ficha pública: información del vehículo y contacto del vendedor.
func (h *Handlers) DetalleVehiculo(c *gin.Context) {
	v, contacto, err := h.api.Vehiculo(peticion(c), c.Param("id"))
	if err != nil {
		fallo(c, err, "/")
		return
	}
	vendedor := cliente.Usuario{Nombre: contacto.Nombre, Email: contacto.Email, Telefono: contacto.Telefono}
	render(c, http.StatusOK, "detalle.html", gin.H{"Vehiculo": v, "Vendedor": vendedor, "Admin": false})
}

// DetalleAdmin muestra al administrador la publicación completa (cualquier estado) y la cuenta del vendedor.
func (h *Handlers) DetalleAdmin(c *gin.Context) {
	v, vendedor, err := h.api.DetallePublicacion(peticion(c), c.Param("id"))
	if err != nil {
		fallo(c, err, "/panel")
		return
	}
	render(c, http.StatusOK, "detalle.html", gin.H{"Vehiculo": v, "Vendedor": vendedor, "Admin": true})
}

func filtrar(vehiculos []cliente.Vehiculo, texto string, anioDesde int) []cliente.Vehiculo {
	texto = strings.ToLower(texto)
	visibles := []cliente.Vehiculo{}
	for _, v := range vehiculos {
		buscable := strings.ToLower(v.Titulo + " " + v.Marca)
		if strings.Contains(buscable, texto) && v.Anio >= anioDesde {
			visibles = append(visibles, v)
		}
	}
	return visibles
}

// ---------- Ingreso y registro ----------

func (h *Handlers) FormIngresar(c *gin.Context) {
	render(c, http.StatusOK, "acceso.html", gin.H{"Registro": false})
}

func (h *Handlers) FormRegistro(c *gin.Context) {
	render(c, http.StatusOK, "acceso.html", gin.H{"Registro": true})
}

func (h *Handlers) Ingresar(c *gin.Context) {
	if err := h.iniciarSesion(c, c.PostForm("email"), c.PostForm("clave")); err != nil {
		fallo(c, err, "/ingresar")
		return
	}
	redirigir(c, "/panel")
}

func (h *Handlers) Registrar(c *gin.Context) {
	nuevo := cliente.NuevoUsuario{
		Nombre:   strings.TrimSpace(c.PostForm("nombre")),
		Email:    strings.TrimSpace(c.PostForm("email")),
		Telefono: strings.TrimSpace(c.PostForm("telefono")),
		Clave:    c.PostForm("clave"),
	}
	if err := h.api.Registrar(peticion(c), nuevo); err != nil {
		fallo(c, err, "/registro")
		return
	}
	if err := h.iniciarSesion(c, nuevo.Email, nuevo.Clave); err != nil {
		fallo(c, err, "/ingresar")
		return
	}
	avisar(c, false, "Cuenta creada. Ya puedes publicar tu vehículo.")
	redirigir(c, "/panel")
}

func (h *Handlers) iniciarSesion(c *gin.Context, email, clave string) error {
	tok, _, err := h.api.IniciarSesion(peticion(c), strings.TrimSpace(email), clave)
	if err != nil {
		return err
	}
	fijarCookie(c, cookieSesion, tok, duracionSeg)
	return nil
}

func (h *Handlers) Salir(c *gin.Context) {
	fijarCookie(c, cookieSesion, "", -1)
	avisar(c, false, "Sesión cerrada.")
	redirigir(c, "/")
}

// ---------- Panel (vendedor o administrador según el rol) ----------

func (h *Handlers) Panel(c *gin.Context) {
	if usuarioActual(c).EsAdmin() {
		h.panelAdmin(c)
		return
	}
	h.panelVendedor(c)
}

// HU-VEN-01
func (h *Handlers) panelVendedor(c *gin.Context) {
	vehiculos, err := h.api.MisVehiculos(peticion(c))
	if err != nil {
		fallo(c, err, "/")
		return
	}
	render(c, http.StatusOK, "vendedor.html", gin.H{
		"Vehiculos": vehiculos,
		"AnioMax":   time.Now().Year() + 1,
		"Marcas":    marcas,
		"MaxFotos":  maxFotos,
		"Borrador":  h.borradores.Obtener(usuarioActual(c).ID),
	})
}

// guardarBorrador lee el formulario de publicación (campos y fotos nuevas) y lo guarda
// en el borrador del vendedor. Lo usan todos los botones del formulario.
func (h *Handlers) guardarBorrador(c *gin.Context) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFotos*maxBytesFoto+1<<20)
	form, err := c.MultipartForm()
	if err != nil {
		return errFoto
	}
	campos := CamposVehiculo{
		Titulo:      strings.TrimSpace(c.PostForm("titulo")),
		Marca:       strings.TrimSpace(c.PostForm("marca")),
		Anio:        strings.TrimSpace(c.PostForm("anio")),
		Precio:      strings.TrimSpace(c.PostForm("precio")),
		Descripcion: strings.TrimSpace(c.PostForm("descripcion")),
	}
	var nuevas []cliente.Archivo
	for _, fh := range form.File["fotos"] {
		if fh.Size == 0 { // el navegador envía un archivo vacío si no se eligió ninguno
			continue
		}
		f, err := fh.Open()
		if err != nil {
			return err
		}
		contenido, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
		nuevas = append(nuevas, cliente.Archivo{Nombre: fh.Filename, Contenido: contenido})
	}
	return h.borradores.Actualizar(usuarioActual(c).ID, campos, nuevas)
}

// CargarFotos guarda el borrador para que el vendedor vea la vista previa de sus fotos.
func (h *Handlers) CargarFotos(c *gin.Context) {
	if err := h.guardarBorrador(c); err != nil {
		avisar(c, true, err.Error())
	} else {
		avisar(c, false, "Fotos cargadas. Revisa la vista previa y envía la publicación.")
	}
	redirigir(c, "/panel#publicar")
}

// QuitarFoto conserva lo escrito en el formulario y descarta una foto del borrador.
func (h *Handlers) QuitarFoto(c *gin.Context) {
	if err := h.guardarBorrador(c); err != nil {
		avisar(c, true, err.Error())
	}
	h.borradores.QuitarFoto(usuarioActual(c).ID, c.Param("id"))
	redirigir(c, "/panel#publicar")
}

// FotoBorrador muestra una foto aún no publicada; solo la ve el vendedor que la cargó.
func (h *Handlers) FotoBorrador(c *gin.Context) {
	f, ok := h.borradores.Foto(usuarioActual(c).ID, c.Param("id"))
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, f.Tipo, f.Contenido)
}

// Publicar envía al backend el borrador completo (campos + fotos cargadas).
func (h *Handlers) Publicar(c *gin.Context) {
	if err := h.guardarBorrador(c); err != nil {
		avisar(c, true, err.Error())
		redirigir(c, "/panel#publicar")
		return
	}
	usuarioID := usuarioActual(c).ID
	br := h.borradores.Obtener(usuarioID)
	anio, _ := strconv.Atoi(br.Campos.Anio)
	pesos, _ := strconv.ParseInt(br.Campos.Precio, 10, 64)
	err := h.api.Publicar(peticion(c), cliente.NuevoVehiculo{
		Titulo:         br.Campos.Titulo,
		Marca:          br.Campos.Marca,
		Anio:           anio,
		PrecioCentavos: pesos * 100,
		Descripcion:    br.Campos.Descripcion,
		Fotos:          br.Archivos(),
	})
	if err != nil {
		fallo(c, err, "/panel#publicar")
		return
	}
	h.borradores.Descartar(usuarioID)
	avisar(c, false, "Vehículo enviado. Aparecerá en el catálogo cuando sea aprobado.")
	redirigir(c, "/panel")
}

// Foto reenvía al navegador una imagen almacenada en el backend.
func (h *Handlers) Foto(c *gin.Context) {
	resp, err := h.api.Foto(peticion(c), c.Param("nombre"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()
	c.Header("Cache-Control", "private, max-age=3600")
	c.DataFromReader(http.StatusOK, resp.ContentLength, resp.Header.Get("Content-Type"), io.Reader(resp.Body), nil)
}

// HU-ADM-01
func (h *Handlers) panelAdmin(c *gin.Context) {
	p := peticion(c)
	vehiculos, err := h.api.TodosLosVehiculos(p)
	if err != nil {
		fallo(c, err, "/")
		return
	}
	usuarios, err := h.api.Usuarios(p)
	if err != nil {
		fallo(c, err, "/")
		return
	}
	render(c, http.StatusOK, "admin.html", gin.H{"Vehiculos": vehiculos, "Usuarios": usuarios})
}

// Accion adapta las operaciones sobre un recurso (:id) que terminan volviendo al panel.
func (h *Handlers) Accion(op func(cliente.Peticion, string) error, mensajeExito string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := op(peticion(c), c.Param("id")); err != nil {
			fallo(c, err, "/panel")
			return
		}
		avisar(c, false, mensajeExito)
		redirigir(c, "/panel")
	}
}
