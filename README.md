# AutoMarket — Frontend

Interfaz web estática de AutoMarket. Este repositorio contiene únicamente el
frontend; el servicio REST debe ejecutarse y desplegarse por separado.

## Interfaz

- Catálogo público con búsqueda por marca, modelo y año.
- Registro e inicio de sesión de vendedores.
- Panel de vendedor para publicar vehículos y marcar sus publicaciones como
  vendidas.
- Panel de administrador para aprobar o eliminar publicaciones y administrar
  cuentas.
- Diseño adaptable a móviles y escritorio.

## Desarrollo local

Sirve la carpeta `web` con cualquier servidor de archivos estáticos. En VS Code
se puede abrir `web/index.html` con Live Server. La URL que se usa
para abrir la aplicación depende de la herramienta elegida.

## Conexión al backend

Edita `web/static/config.js` y asigna la URL base del backend:

```js
window.AUTOMARKET_API_BASE = "https://api.example.com";
```

El frontend añade a esa URL las rutas `/api/...`. Si el frontend y el backend
usan el mismo origen, deja el valor vacío. Para usarlos en orígenes distintos,
el backend debe permitir mediante CORS el origen del frontend y los encabezados
`Authorization` y `Content-Type`.

El backend conectado debe implementar los endpoints de usuario, sesión,
vehículos, vendedor y administración consumidos por `web/static/app.js`. Este
repositorio no incluye servidor ni almacenamiento de datos.

## Publicación

Despliega el contenido de `web` en un hosting de sitios estáticos y configura
`web/static/config.js` con la URL pública del backend. No incluyas credenciales
ni tokens en ese archivo: los tokens de sesión se mantienen en el almacenamiento
de sesión del navegador.
