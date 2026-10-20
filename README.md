# AutoMarket

Interfaz web para consultar, publicar y administrar anuncios de vehículos, escrita en
**Go + Gin** (sin JavaScript). Las páginas se generan en el servidor con
`html/template` y los datos se obtienen del backend REST separado
[AutoMarket-Phantom-Back](https://github.com/JuanferCalle/AutoMarket-Phantom-Back).
El navegador solo recibe HTML y CSS; las acciones son formularios (POST → redirección).

## Ejecutar

Primero inicia el backend AutoMarket-Phantom-Back (por ejemplo,
`go run ./cmd/server` desde su repositorio). Luego, desde este repositorio:

```bash
go run .     # http://localhost:3000
```

| Variable | Valor predeterminado |
|---|---|
| `PUERTO` | `3000` |
| `BACK_URL` | `http://localhost:8080` |

## Funcionalidades

- Catálogo público con búsqueda por marca, título del anuncio y año, y página de
  detalle de cada vehículo con sus fotos y el contacto del vendedor.
- Galerías de fotos desplazables y vista ampliada a pantalla completa (solo CSS).
- Registro e inicio de sesión de vendedores, con opción para ver la contraseña.
- Panel de vendedor para publicar vehículos con fotos (vista previa antes de enviar),
  consultar el estado de sus anuncios y marcar vehículos publicados como vendidos.
- Panel de administrador para revisar el detalle de cada publicación, aprobarla o
  eliminarla, y administrar cuentas.
- Diseño adaptable a móviles y escritorio.

La aprobación de un anuncio gestiona su publicación en el sistema; no certifica
la condición mecánica ni legal del vehículo.

## Estructura

```text
main.go                 arranque Gin; incrusta web/ con embed
internal/cliente/       cliente HTTP de la API del backend (net/http)
internal/web/           router Gin, handlers, sesión, borradores de fotos y funciones de plantilla
web/templates/          páginas: inicio, detalle, acceso, vendedor, admin y parciales
web/static/styles.css   estilos
```

## Conexión y sesiones

El front llama a la API desde su servidor usando `BACK_URL`; el navegador nunca se
comunica directamente con el backend, por lo que no se requiere CORS. El front
reenvía la IP del navegador en `X-Forwarded-For` para la auditoría del backend.

El token de la API se guarda en la cookie `automarket_sesion` (`HttpOnly`,
`SameSite=Lax`) y se envía como `Authorization: Bearer`. La autorización final se
valida siempre en el backend.
