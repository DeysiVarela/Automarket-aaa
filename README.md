# AutoMarket

Interfaz web para consultar, publicar y administrar anuncios de vehículos. El
repositorio incluye dos presentaciones que consumen el backend REST separado
[AutoMarket-Phantom-Back](https://github.com/JuanferCalle/AutoMarket-Phantom-Back):

- Una aplicación Go + Gin con páginas renderizadas mediante `html/template`.
- Una interfaz estática con HTML, CSS y JavaScript, que ofrece búsqueda y
  galerías navegables de fotos.

## Ejecutar

Primero inicia el backend AutoMarket-Phantom-Back (por ejemplo,
`go run ./cmd/server -demo` desde su repositorio). Luego, desde este repositorio,
inicia la aplicación Gin:

```bash
go run .
```

Abre `http://localhost:3000/` para la interfaz Gin o
`http://localhost:3000/spa/` para la interfaz estática integrada.

| Variable | Valor predeterminado |
|---|---|
| `PUERTO` | `3000` |
| `BACK_URL` | `http://localhost:8080` |

Para servir la interfaz estática de forma independiente, sirve la carpeta `web`
con un servidor de archivos estáticos y abre `web/index.html`.

## Funcionalidades

- Catálogo público con búsqueda por marca, título del anuncio y año.
- Galerías de fotos desplazables con el gesto horizontal o las flechas del
  teclado.
- Registro e inicio de sesión de vendedores.
- Panel de vendedor para publicar vehículos, consultar el estado de sus anuncios
  y marcar vehículos publicados como vendidos.
- Panel de administrador para revisar y aprobar publicaciones y administrar
  cuentas.
- Diseño adaptable a móviles y escritorio.

La aprobación de un anuncio gestiona su publicación en el sistema; no certifica
la condición mecánica ni legal del vehículo.

## Estructura

```text
main.go                 arranque Gin; incrusta web/ con embed
internal/cliente/       cliente HTTP de la API del backend
internal/web/           router Gin, handlers, sesión y funciones de plantilla
web/templates/          páginas renderizadas por Gin
web/index.html          interfaz estática
web/static/app.js       lógica de la interfaz estática
web/static/styles.css   estilos compartidos por ambas interfaces
web/static/config.js    URL base de la API para la interfaz estática
```

## Conexión y sesiones

La interfaz Gin usa `BACK_URL` para conectarse al backend. La interfaz estática
usa `web/static/config.js`, que por defecto apunta a `http://localhost:8080`:

```js
window.AUTOMARKET_API_BASE = "https://api.example.com";
```

Cuando la interfaz estática se sirve desde un origen diferente al backend, este
debe permitir dicho origen mediante CORS junto con los encabezados
`Authorization` y `Content-Type`. La interfaz Gin guarda el token de la API en
una cookie `HttpOnly` y lo envía como `Authorization: Bearer`. La interfaz
estática guarda su token en el almacenamiento de sesión del navegador. En ambos
casos, la autorización final se valida en el backend.

El backend conserva los usuarios y vehículos; este repositorio implementa las
interfaces y su conexión con la API.
