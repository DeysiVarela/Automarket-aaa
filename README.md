# AutoMarket — Frontend

Interfaz web de AutoMarket escrita en **Go + Gin** (sin JavaScript). Las páginas se
generan en el servidor con `html/template` y los datos se obtienen de la API REST del
backend ([AutoMarket-Phantom-Back](https://github.com/JuanferCalle/AutoMarket-Phantom-Back)).
El navegador solo recibe HTML y CSS; las acciones son formularios (POST → redirección).

## Ejecutar

Primero el backend (`go run ./cmd/server -demo` en su repo), luego:

```bash
go run .     # http://localhost:3000
```

| Variable   | Por defecto             |
|------------|-------------------------|
| `PUERTO`   | `3000`                  |
| `BACK_URL` | `http://localhost:8080` |

## Estructura

```text
main.go                 arranque; incrusta web/ con embed
internal/cliente/       cliente de la API del backend (net/http)
internal/web/           router Gin, handlers, sesión por cookie y funciones de plantilla
web/templates/          páginas: inicio, acceso, vendedor, admin y parciales
web/static/styles.css   estilos
```

## Páginas

| Ruta | Rol | Uso |
|---|---|---|
| `/` | Visitante | Catálogo con búsqueda por marca/modelo y año, con contacto del vendedor |
| `/ingresar`, `/registro` | Todos | Inicio de sesión y registro de vendedores |
| `/panel` | Vendedor | Publicar vehículos y marcarlos como vendidos |
| `/panel` | Administrador | Aprobar o eliminar publicaciones y eliminar cuentas |

## Sesión

El token que entrega el backend se guarda en la cookie `automarket_sesion`
(`HttpOnly`, `SameSite=Lax`) y el front lo envía como `Authorization: Bearer` en cada
llamada a la API. La autorización por rol la decide siempre el backend. El front
reenvía la IP del navegador en `X-Forwarded-For` para la auditoría.
