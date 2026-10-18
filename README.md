# AutoMarket — Frontend

Interfaz web estática de AutoMarket. Este repositorio contiene únicamente el
frontend; el servicio REST debe ejecutarse y desplegarse por separado.

## Interfaz

- Catálogo público con búsqueda por marca, título del anuncio y año.
- Las publicaciones con varias fotos permiten desplazarse horizontalmente por
  su galería desde la tarjeta del catálogo con las teclas de flecha izquierda y
  derecha o con el gesto horizontal en una pantalla táctil.
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

La configuración local de `web/static/config.js` apunta a `http://localhost:8080`,
el puerto predeterminado del backend AutoMarket-Phantom-Back. Para otra URL,
edita ese archivo:

```js
window.AUTOMARKET_API_BASE = "https://api.example.com";
```

El frontend añade a esa URL las rutas `/api/...`. El API local no habilita CORS
por defecto: el navegador bloqueará las peticiones entre orígenes mientras el
backend no permita el origen exacto donde se sirve el frontend, junto con los
encabezados `Authorization` y `Content-Type`. Abrir el archivo como `file://`
tampoco sustituye ese permiso. No se agregó CORS ni se modificó el repositorio
del backend.

El formulario de publicación usa `multipart/form-data` con hasta cinco fotos
JPG, PNG o WebP (5 MB por foto), como espera el backend. El catálogo muestra la
marca, el año, el contacto del vendedor y la primera foto disponible. El
contrato completo de endpoints está documentado en el README del backend.
Este repositorio no incluye servidor ni almacenamiento de datos.

## Publicación

Despliega el contenido de `web` en un hosting de sitios estáticos y configura
`web/static/config.js` con la URL pública del backend. No incluyas credenciales
ni tokens en ese archivo: los tokens de sesión se mantienen en el almacenamiento
de sesión del navegador.
