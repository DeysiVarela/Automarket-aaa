package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/DeysiVarela/Automarket-aaa/internal/cliente"
)

const (
	maxFotos         = 5
	maxBytesFoto     = 5 << 20
	vigenciaBorrador = 2 * time.Hour
)

// tiposImagen: solo se aceptan (y se sirven de vuelta al navegador) imágenes reales,
// detectadas por contenido. Así un archivo HTML disfrazado nunca se entrega como página.
var tiposImagen = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

var errFoto = errors.New("se permiten hasta 5 fotos JPG, PNG o WebP de máximo 5 MB cada una")

// CamposVehiculo conserva lo que el vendedor escribió, tal cual, para volver a mostrarlo.
type CamposVehiculo struct {
	Titulo, Marca, Anio, Precio, Descripcion string
}

type FotoBorrador struct {
	ID        string
	Nombre    string
	Tipo      string
	Contenido []byte
}

// Borrador es la publicación que el vendedor está armando antes de enviarla a revisión.
type Borrador struct {
	Campos      CamposVehiculo
	Fotos       []FotoBorrador
	actualizado time.Time
}

// Borradores guarda en memoria un borrador por usuario. Expiran tras vigenciaBorrador sin uso.
type Borradores struct {
	mu         sync.Mutex
	porUsuario map[string]*Borrador
}

func NuevosBorradores() *Borradores {
	return &Borradores{porUsuario: map[string]*Borrador{}}
}

// tomar devuelve (creándolo si hace falta) el borrador del usuario. Requiere b.mu tomado.
func (b *Borradores) tomar(usuarioID string) *Borrador {
	ahora := time.Now()
	for id, br := range b.porUsuario {
		if ahora.Sub(br.actualizado) > vigenciaBorrador {
			delete(b.porUsuario, id)
		}
	}
	br, ok := b.porUsuario[usuarioID]
	if !ok {
		br = &Borrador{}
		b.porUsuario[usuarioID] = br
	}
	br.actualizado = ahora
	return br
}

// Obtener devuelve una copia del borrador para mostrarlo.
func (b *Borradores) Obtener(usuarioID string) Borrador {
	b.mu.Lock()
	defer b.mu.Unlock()
	br := b.tomar(usuarioID)
	return Borrador{Campos: br.Campos, Fotos: append([]FotoBorrador{}, br.Fotos...)}
}

// Actualizar guarda los campos escritos y agrega las fotos nuevas, validando cada una.
// Las fotos válidas se conservan aunque otra falle.
func (b *Borradores) Actualizar(usuarioID string, campos CamposVehiculo, nuevas []cliente.Archivo) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	br := b.tomar(usuarioID)
	br.Campos = campos
	var err error
	for _, a := range nuevas {
		tipo := http.DetectContentType(a.Contenido)
		if len(br.Fotos) >= maxFotos || len(a.Contenido) > maxBytesFoto || !tiposImagen[tipo] {
			err = errFoto
			continue
		}
		br.Fotos = append(br.Fotos, FotoBorrador{ID: idAleatorio(), Nombre: a.Nombre, Tipo: tipo, Contenido: a.Contenido})
	}
	return err
}

func (b *Borradores) Foto(usuarioID, fotoID string) (FotoBorrador, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, f := range b.tomar(usuarioID).Fotos {
		if f.ID == fotoID {
			return f, true
		}
	}
	return FotoBorrador{}, false
}

func (b *Borradores) QuitarFoto(usuarioID, fotoID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	br := b.tomar(usuarioID)
	for i, f := range br.Fotos {
		if f.ID == fotoID {
			br.Fotos = append(br.Fotos[:i], br.Fotos[i+1:]...)
			return
		}
	}
}

// Descartar elimina el borrador una vez publicado.
func (b *Borradores) Descartar(usuarioID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.porUsuario, usuarioID)
}

func (br Borrador) Archivos() []cliente.Archivo {
	archivos := make([]cliente.Archivo, len(br.Fotos))
	for i, f := range br.Fotos {
		archivos[i] = cliente.Archivo{Nombre: f.Nombre, Contenido: f.Contenido}
	}
	return archivos
}

func idAleatorio() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
