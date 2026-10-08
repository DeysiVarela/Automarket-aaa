# AutoMarket AAA API

Backend de referencia para el taller de implementación de Authentication,
Authorization y Accounting (AAA). La única dependencia externa directa es Gin;
el resto usa la biblioteca estándar de Go.

## Requisitos y ejecución

- Go 1.22 o posterior.
- PowerShell en Windows o una terminal equivalente.

Configura estas variables antes de iniciar el servidor:

| Variable | Requisito | Uso |
| --- | --- | --- |
| `ADMIN_EMAIL` | Dirección de correo válida | Cuenta administradora inicial |
| `ADMIN_PASSWORD` | Entre 12 y 1024 bytes | Contraseña inicial del administrador |
| `AUDIT_KEY` | Al menos 32 bytes aleatorios | Clave HMAC de la bitácora; debe conservarse |
| `AUDIT_LOG_PATH` | Opcional, predeterminado `audit.log` | Ruta del archivo de auditoría |
| `PORT` | Opcional, predeterminado `8080` | Puerto HTTP |

Desde PowerShell:

```powershell
$env:ADMIN_EMAIL = "admin@automarket.local"
$env:ADMIN_PASSWORD = "cambia-esta-clave-por-una-segura"
$env:AUDIT_KEY = "reemplaza-por-una-clave-aleatoria-de-al-menos-32-bytes"
go mod tidy
go run .
```

La cuenta administradora se crea al arrancar. Por decisión de alcance, usuarios,
vehículos y sesiones viven en memoria y se pierden al reiniciar. La bitácora sí
se conserva en disco. El servidor se detiene si la bitácora no pasa la
verificación HMAC; no cambies `AUDIT_KEY` mientras conserves ese archivo.

## API

Todas las rutas usan el prefijo `/api`. Los cuerpos JSON rechazan propiedades no
reconocidas y tienen un límite de 1 MiB.

| Método y ruta | Acceso | Función |
| --- | --- | --- |
| `POST /api/users` | Público | Registrar vendedor |
| `POST /api/sessions` | Público | Iniciar sesión y obtener token Bearer |
| `GET /api/vehicles` | Público | Listar publicaciones aprobadas |
| `GET /api/vehicles/:id` | Público | Ver vehículo publicado y contacto del vendedor |
| `POST /api/vehicles` | Vendedor | Crear publicación en estado `pending` |
| `POST /api/vehicles/:id/sold` | Dueño vendedor | Reportar venta |
| `GET /api/admin/vehicles/pending` | Administrador | Consultar publicaciones por aprobar |
| `POST /api/admin/vehicles/:id/approve` | Administrador | Aprobar publicación pendiente |
| `DELETE /api/admin/vehicles/:id` | Administrador | Eliminar publicación |
| `DELETE /api/admin/users/:id` | Administrador | Eliminar usuario y sus publicaciones |

Para las rutas protegidas, envía `Authorization: Bearer <access_token>`. Los
tokens aleatorios vencen a las 8 horas. El registro asigna siempre el rol
`seller`; el cliente no puede asignarse privilegios administrativos. No se
incluye una ruta para promover usuarios.

Ejemplo de registro:

```json
{
  "name": "Ana Pérez",
  "email": "ana@example.com",
  "phone": "+57 300 000 0000",
  "password": "una-clave-segura"
}
```

Ejemplo de publicación:

```json
{
  "title": "Sedán usado en buen estado",
  "make": "Toyota",
  "model": "Corolla",
  "year": 2022,
  "price_cents": 8500000000,
  "description": "Mantenimiento al día."
}
```

`price_cents` es un entero en centavos de la moneda que defina el equipo. El
catálogo solo expone vehículos en estado `published`, e incluye nombre, correo
y teléfono del vendedor para cumplir la historia del visitante.

## AAA implementado

- **Authentication:** registro e inicio de sesión con tokens aleatorios Bearer;
  contraseña derivada con PBKDF2-HMAC-SHA-256 usando `crypto` estándar, sal
  aleatoria y comparación en tiempo constante.
- **Authorization:** middleware de Gin verifica identidad y rol; las
  operaciones de vendedor comprueban además propiedad del vehículo. La
  autorización también se valida al modificar datos.
- **Accounting:** cada solicitud, incluso las rechazadas, produce un registro
  JSON sin cuerpo ni credenciales. Los registros se encadenan con HMAC-SHA-256,
  se sincronizan al disco y se verifican al iniciar.

El taller exige HTTP sin TLS para esta entrega. Por ello, no expongas este
servidor a redes públicas ni uses credenciales reales; HTTP transmite los
tokens sin cifrar.
