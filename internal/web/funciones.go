package web

import (
	"html/template"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// funciones disponibles dentro de las plantillas.
var funciones = template.FuncMap{
	"precio": precio,
	"estado": func(e string) string {
		etiquetas := map[string]string{"pending": "En revisión", "published": "Publicado", "sold": "Vendido"}
		if t, ok := etiquetas[e]; ok {
			return t
		}
		return e
	},
	"fecha": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Local().Format("02/01/2006 15:04")
	},
	"telefonoIntl": telefonoIntl,
	"sumar":        func(a, b int) int { return a + b },
}

// precio convierte centavos a pesos con separador de miles: 7800000000 -> "$ 78.000.000".
func precio(centavos int64) string {
	digitos := strconv.FormatInt(centavos/100, 10)
	salida := []byte{}
	for i, d := range []byte(digitos) {
		if i > 0 && (len(digitos)-i)%3 == 0 {
			salida = append(salida, '.')
		}
		salida = append(salida, d)
	}
	return "$ " + string(salida)
}

// telefonoIntl deja solo dígitos y agrega el indicativo de Colombia a los celulares de 10 dígitos,
// para los enlaces tel: y wa.me: "+57 300 123 4567" -> "573001234567".
func telefonoIntl(telefono string) string {
	digitos := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, telefono)
	if len(digitos) == 10 {
		return "57" + digitos
	}
	return digitos
}
