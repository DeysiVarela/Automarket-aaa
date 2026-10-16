// Package cliente consume la API REST del backend de AutoMarket (solo net/http).
package cliente

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Contacto struct {
	Nombre   string `json:"name"`
	Email    string `json:"email"`
	Telefono string `json:"phone"`
}

type Vehiculo struct {
	ID             string    `json:"id"`
	Titulo         string    `json:"title"`
	Marca          string    `json:"make"`
	Anio           int       `json:"year"` // el "modelo" del vehículo
	PrecioCentavos int64     `json:"price_cents"`
	Descripcion    string    `json:"description"`
	Fotos          []string  `json:"photos"`
	Estado         string    `json:"state"`
	CreadoEn       time.Time `json:"created_at"`
	Vendedor       *Contacto `json:"seller,omitempty"`
}

type Usuario struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"name"`
	Email    string    `json:"email"`
	Telefono string    `json:"phone"`
	Rol      string    `json:"role"`
	CreadoEn time.Time `json:"created_at"`
}

func (u Usuario) EsAdmin() bool { return u.Rol == "admin" }

type NuevoUsuario struct {
	Nombre   string `json:"name"`
	Email    string `json:"email"`
	Telefono string `json:"phone"`
	Clave    string `json:"password"`
}

type NuevoVehiculo struct {
	Titulo         string
	Marca          string
	Anio           int
	PrecioCentavos int64
	Descripcion    string
	Fotos          []Archivo
}

// Archivo es una foto ya cargada en memoria, lista para reenviarse al backend.
type Archivo struct {
	Nombre    string
	Contenido []byte
}

// ErrorAPI representa una respuesta de error del backend ({"error": "..."}).
type ErrorAPI struct {
	Estado  int
	Mensaje string
}

func (e *ErrorAPI) Error() string { return e.Mensaje }

// Peticion identifica a quién se atiende: token de sesión (puede ser vacío) e IP del
// navegador, que se reenvía en X-Forwarded-For para la auditoría del backend.
type Peticion struct {
	Ctx   context.Context
	Token string
	IP    string
}

type API struct {
	base string
	http *http.Client
}

func Nueva(base string) *API {
	return &API{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

// hacer envía un cuerpo JSON (opcional) y decodifica la respuesta JSON en destino.
func (a *API) hacer(p Peticion, metodo, ruta string, cuerpo, destino any) error {
	var lector io.Reader
	tipo := ""
	if cuerpo != nil {
		b, err := json.Marshal(cuerpo)
		if err != nil {
			return err
		}
		lector, tipo = bytes.NewReader(b), "application/json"
	}
	resp, err := a.enviar(p, metodo, ruta, lector, tipo)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if destino == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(destino)
}

// enviar hace la petición con el token y la IP del navegador. Si el backend responde
// con error, lo devuelve como *ErrorAPI; si no, el llamador debe cerrar resp.Body.
func (a *API) enviar(p Peticion, metodo, ruta string, cuerpo io.Reader, tipo string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(p.Ctx, metodo, a.base+ruta, cuerpo)
	if err != nil {
		return nil, err
	}
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	if p.IP != "" {
		req.Header.Set("X-Forwarded-For", p.IP)
	}

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar al backend: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = http.StatusText(resp.StatusCode)
		}
		return nil, &ErrorAPI{Estado: resp.StatusCode, Mensaje: e.Error}
	}
	return resp, nil
}

func id(v string) string { return url.PathEscape(v) }

// ---------- Público ----------

func (a *API) Catalogo(p Peticion) ([]Vehiculo, error) {
	var r struct{ Vehicles []Vehiculo }
	return r.Vehicles, a.hacer(p, http.MethodGet, "/api/vehicles", nil, &r)
}

func (a *API) Registrar(p Peticion, u NuevoUsuario) error {
	return a.hacer(p, http.MethodPost, "/api/users", u, nil)
}

func (a *API) IniciarSesion(p Peticion, email, clave string) (string, Usuario, error) {
	var r struct {
		Token   string  `json:"access_token"`
		Usuario Usuario `json:"user"`
	}
	err := a.hacer(p, http.MethodPost, "/api/sessions", map[string]string{"email": email, "password": clave}, &r)
	return r.Token, r.Usuario, err
}

// Vehiculo consulta un vehículo publicado y el contacto de su vendedor.
func (a *API) Vehiculo(p Peticion, vehiculoID string) (Vehiculo, Contacto, error) {
	var r struct {
		Vehiculo Vehiculo `json:"vehicle"`
		Vendedor Contacto `json:"seller"`
	}
	err := a.hacer(p, http.MethodGet, "/api/vehicles/"+id(vehiculoID), nil, &r)
	return r.Vehiculo, r.Vendedor, err
}

func (a *API) UsuarioActual(p Peticion) (Usuario, error) {
	var u Usuario
	return u, a.hacer(p, http.MethodGet, "/api/me", nil, &u)
}

// ---------- Vendedor ----------

// Publicar reenvía al backend el formulario como multipart/form-data, fotos incluidas.
func (a *API) Publicar(p Peticion, v NuevoVehiculo) error {
	var cuerpo bytes.Buffer
	w := multipart.NewWriter(&cuerpo)
	campos := map[string]string{
		"title":       v.Titulo,
		"make":        v.Marca,
		"year":        strconv.Itoa(v.Anio),
		"price_cents": strconv.FormatInt(v.PrecioCentavos, 10),
		"description": v.Descripcion,
	}
	for nombre, valor := range campos {
		if err := w.WriteField(nombre, valor); err != nil {
			return err
		}
	}
	for _, f := range v.Fotos {
		destino, err := w.CreateFormFile("photos", f.Nombre)
		if err != nil {
			return err
		}
		if _, err := destino.Write(f.Contenido); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}

	resp, err := a.enviar(p, http.MethodPost, "/api/vehicles", &cuerpo, w.FormDataContentType())
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Foto descarga una imagen del backend; el llamador debe cerrar el cuerpo.
func (a *API) Foto(p Peticion, nombre string) (*http.Response, error) {
	return a.enviar(p, http.MethodGet, "/api/photos/"+id(nombre), nil, "")
}

func (a *API) MisVehiculos(p Peticion) ([]Vehiculo, error) {
	var r struct{ Vehicles []Vehiculo }
	return r.Vehicles, a.hacer(p, http.MethodGet, "/api/my/vehicles", nil, &r)
}

func (a *API) MarcarVendido(p Peticion, vehiculoID string) error {
	return a.hacer(p, http.MethodPost, "/api/vehicles/"+id(vehiculoID)+"/sold", nil, nil)
}

// ---------- Administrador ----------

func (a *API) TodosLosVehiculos(p Peticion) ([]Vehiculo, error) {
	var r struct{ Vehicles []Vehiculo }
	return r.Vehicles, a.hacer(p, http.MethodGet, "/api/admin/vehicles", nil, &r)
}

// DetallePublicacion trae una publicación en cualquier estado con la cuenta de su vendedor.
func (a *API) DetallePublicacion(p Peticion, vehiculoID string) (Vehiculo, Usuario, error) {
	var r struct {
		Vehiculo Vehiculo `json:"vehicle"`
		Vendedor Usuario  `json:"seller"`
	}
	err := a.hacer(p, http.MethodGet, "/api/admin/vehicles/"+id(vehiculoID), nil, &r)
	return r.Vehiculo, r.Vendedor, err
}

func (a *API) Aprobar(p Peticion, vehiculoID string) error {
	return a.hacer(p, http.MethodPost, "/api/admin/vehicles/"+id(vehiculoID)+"/approve", nil, nil)
}

func (a *API) EliminarVehiculo(p Peticion, vehiculoID string) error {
	return a.hacer(p, http.MethodDelete, "/api/admin/vehicles/"+id(vehiculoID), nil, nil)
}

func (a *API) Usuarios(p Peticion) ([]Usuario, error) {
	var r struct{ Users []Usuario }
	return r.Users, a.hacer(p, http.MethodGet, "/api/admin/users", nil, &r)
}

func (a *API) EliminarUsuario(p Peticion, usuarioID string) error {
	return a.hacer(p, http.MethodDelete, "/api/admin/users/"+id(usuarioID), nil, nil)
}
