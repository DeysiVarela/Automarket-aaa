(() => {
  const tokenKey = "automarket-token";
  const grid = document.querySelector("#vehicle-grid");
  const workspace = document.querySelector("#workspace");
  const workspaceContent = document.querySelector("#workspace-content");
  const dialog = document.querySelector("#auth-dialog");
  const notice = document.querySelector("#notice");
  const searchInput = document.querySelector("#search-input");
  const yearInput = document.querySelector("#year-input");
  let currentUser = null;
  let vehicles = [];
  let authMode = "login";

  const token = () => sessionStorage.getItem(tokenKey);
  const money = (cents) => new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    maximumFractionDigits: 0,
  }).format(cents / 100);

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (options.body) headers.set("Content-Type", "application/json");
    if (token()) headers.set("Authorization", `Bearer ${token()}`);
    const response = await fetch(path, { ...options, headers });
    if (response.status === 204) return null;
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(data.error || "No fue posible completar la solicitud.");
      error.status = response.status;
      throw error;
    }
    return data;
  }

  function showNotice(message, isError = false) {
    notice.textContent = message;
    notice.classList.toggle("error", isError);
    notice.classList.remove("hidden");
    window.setTimeout(() => notice.classList.add("hidden"), 5000);
  }

  function element(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function button(text, className, onClick) {
    const node = element("button", `button ${className}`, text);
    node.type = "button";
    node.addEventListener("click", onClick);
    return node;
  }

  function vehicleCard(item) {
    const card = element("article", "vehicle-card");
    const photo = element("div", "vehicle-photo");
    photo.append(element("span", "photo-tag", "VERIFICADO POR AUTOMARKET"));
    photo.append(element("span", "vehicle-glyph", "🚙"));
    const info = element("div", "vehicle-info");
    info.append(element("h3", "vehicle-title", item.title));
    const meta = element("div", "vehicle-meta");
    meta.append(element("span", "", `${item.make} ${item.model}`));
    meta.append(element("span", "separator"));
    meta.append(element("span", "", String(item.year)));
    info.append(meta);
    info.append(element("p", "vehicle-description", item.description));
    const bottom = element("div", "vehicle-bottom");
    bottom.append(element("span", "vehicle-price", money(item.price_cents)));
    const contact = document.createElement("a");
    contact.className = "seller-link";
    contact.href = `mailto:${encodeURIComponent(item.seller.email)}`;
    contact.textContent = "Contactar vendedor ↗";
    bottom.append(contact);
    info.append(bottom);
    info.append(element("p", "seller-contact", `${item.seller.name} · ${item.seller.phone}`));
    card.append(photo, info);
    return card;
  }

  function renderCatalog() {
    const query = searchInput.value.trim().toLocaleLowerCase("es");
    const year = Number(yearInput.value) || 0;
    const visible = vehicles.filter((item) => {
      const searchable = `${item.title} ${item.make} ${item.model}`.toLocaleLowerCase("es");
      return (!query || searchable.includes(query)) && (!year || item.year >= year);
    });
    grid.replaceChildren(...visible.map(vehicleCard));
    document.querySelector("#result-count").textContent = `${visible.length} ${visible.length === 1 ? "vehículo" : "vehículos"}`;
    document.querySelector("#empty-state").classList.toggle("hidden", visible.length !== 0);
    grid.classList.toggle("hidden", visible.length === 0);
  }

  async function loadCatalog() {
    try {
      const result = await api("/api/vehicles");
      vehicles = result.vehicles || [];
      renderCatalog();
    } catch (error) {
      showNotice(error.message, true);
    }
  }

  function showAuth(mode) {
    authMode = mode;
    const registering = mode === "register";
    document.querySelector("#auth-title").textContent = registering ? "Crea tu cuenta" : "Qué bueno tenerte aquí";
    document.querySelector("#auth-subtitle").textContent = registering
      ? "Regístrate para publicar y administrar tus vehículos."
      : "Ingresa a tu cuenta para continuar.";
    document.querySelector("#auth-eyebrow").textContent = registering ? "ÚNETE A AUTOMARKET" : "BIENVENIDO A AUTOMARKET";
    document.querySelector("#name-field").classList.toggle("hidden", !registering);
    document.querySelector("#phone-field").classList.toggle("hidden", !registering);
    document.querySelector("#name").required = registering;
    document.querySelector("#phone").required = registering;
    document.querySelector("#password").autocomplete = registering ? "new-password" : "current-password";
    document.querySelector("#login-tab").classList.toggle("active", !registering);
    document.querySelector("#register-tab").classList.toggle("active", registering);
    document.querySelector("#auth-submit").textContent = registering ? "Crear cuenta" : "Iniciar sesión";
    document.querySelector("#auth-error").classList.add("hidden");
    if (!dialog.open) dialog.showModal();
  }

  function statusLabel(state) {
    const labels = { pending: "En revisión", published: "Publicado", sold: "Vendido" };
    return labels[state] || state;
  }

  function dashboardRow(item, actions = []) {
    const row = element("div", "dashboard-row");
    const main = element("div", "dashboard-row-main");
    main.append(element("div", "dashboard-row-title", item.title));
    main.append(element("div", "dashboard-row-meta", `${item.make} ${item.model} · ${item.year} · ${money(item.price_cents)}`));
    row.append(main);
    row.append(element("span", `status-pill status-${item.state}`, statusLabel(item.state)));
    if (actions.length) {
      const actionGroup = element("div", "row-actions");
      for (const action of actions) actionGroup.append(action);
      row.append(actionGroup);
    }
    return row;
  }

  function emptyMessage(text) {
    return element("p", "dashboard-empty", text);
  }

  async function loadSellerPanel() {
    workspaceContent.replaceChildren();
    const createPanel = element("section", "panel");
    createPanel.append(element("h3", "", "Publica un vehículo"));
    createPanel.append(element("p", "panel-subtitle", "Tu anuncio quedará pendiente hasta que un administrador lo revise."));
    const form = element("form", "listing-form");
    const fields = [
      ["title", "Título del anuncio", "text", true],
      ["make", "Marca", "text", true],
      ["model", "Modelo", "text", true],
      ["year", "Año", "number", true],
      ["price", "Precio en pesos colombianos", "number", true],
    ];
    for (const [name, label, type, required] of fields) {
      const field = element("label", "form-field", label);
      const input = document.createElement("input");
      input.name = name;
      input.type = type;
      input.required = required;
      if (name === "year") {
        input.min = "1886";
        input.max = String(new Date().getFullYear() + 1);
      }
      if (name === "price") {
        input.min = "1";
        input.step = "1";
      }
      field.append(input);
      form.append(field);
    }
    const descriptionField = element("label", "form-field span-two", "Descripción");
    const description = document.createElement("textarea");
    description.name = "description";
    description.maxLength = 4000;
    description.required = true;
    descriptionField.append(description);
    form.append(descriptionField);
    const formError = element("p", "form-error hidden");
    formError.setAttribute("role", "alert");
    formError.classList.add("span-two");
    form.append(formError);
    const submit = element("button", "button button-dark", "Enviar a revisión");
    submit.type = "submit";
    form.append(submit);
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      formError.classList.add("hidden");
      const values = Object.fromEntries(new FormData(form).entries());
      try {
        await api("/api/vehicles", {
          method: "POST",
          body: JSON.stringify({
            title: values.title,
            make: values.make,
            model: values.model,
            year: Number(values.year),
            price_cents: Math.round(Number(values.price) * 100),
            description: values.description,
          }),
        });
        form.reset();
        showNotice("Vehículo enviado. Aparecerá en el catálogo cuando sea aprobado.");
        await loadSellerPanel();
      } catch (error) {
        formError.textContent = error.message;
        formError.classList.remove("hidden");
      }
    });
    createPanel.append(form);
    workspaceContent.append(createPanel);

    const listPanel = element("section", "panel");
    listPanel.append(element("h3", "", "Mis publicaciones"));
    const list = element("div", "dashboard-list");
    listPanel.append(list);
    workspaceContent.append(listPanel);
    try {
      const result = await api("/api/my/vehicles");
      if (!result.vehicles.length) list.append(emptyMessage("Todavía no tienes vehículos publicados."));
      for (const item of result.vehicles) {
        const actions = [];
        if (item.state === "published") {
          actions.push(button("Marcar vendido", "button-small button-outline", async () => {
            try {
              await api(`/api/vehicles/${encodeURIComponent(item.id)}/sold`, { method: "POST" });
              showNotice("El vehículo se marcó como vendido.");
              await loadSellerPanel();
              await loadCatalog();
            } catch (error) {
              showNotice(error.message, true);
            }
          }));
        }
        list.append(dashboardRow(item, actions));
      }
    } catch (error) {
      list.append(emptyMessage(error.message));
    }
  }

  async function loadAdminPanel() {
    workspaceContent.replaceChildren();
    const pendingPanel = element("section", "panel");
    pendingPanel.append(element("h3", "", "Gestión de publicaciones"));
    const pendingList = element("div", "dashboard-list");
    pendingPanel.append(pendingList);
    workspaceContent.append(pendingPanel);
    const usersPanel = element("section", "panel");
    usersPanel.append(element("h3", "", "Cuentas registradas"));
    usersPanel.append(element("p", "panel-subtitle", "La contraseña y los datos de autenticación nunca se muestran."));
    const usersList = element("div", "dashboard-list");
    usersPanel.append(usersList);
    workspaceContent.append(usersPanel);
    try {
      const result = await api("/api/admin/vehicles");
      if (!result.vehicles.length) pendingList.append(emptyMessage("No hay publicaciones."));
      for (const item of result.vehicles) {
        const actions = [];
        if (item.state === "pending") {
          actions.push(button("Aprobar", "button-small button-dark", async () => {
            try {
              await api(`/api/admin/vehicles/${encodeURIComponent(item.id)}/approve`, { method: "POST" });
              showNotice("Publicación aprobada y disponible en el catálogo.");
              await loadAdminPanel();
              await loadCatalog();
            } catch (error) {
              showNotice(error.message, true);
            }
          }));
        }
        const remove = button("Eliminar", "button-small button-danger", async () => {
          if (!window.confirm(`¿Eliminar la publicación “${item.title}”?`)) return;
          try {
            await api(`/api/admin/vehicles/${encodeURIComponent(item.id)}`, { method: "DELETE" });
            showNotice("Publicación eliminada.");
            await loadAdminPanel();
          } catch (error) {
            showNotice(error.message, true);
          }
        });
        actions.push(remove);
        pendingList.append(dashboardRow(item, actions));
      }
    } catch (error) {
      pendingList.append(emptyMessage(error.message));
    }
    try {
      const result = await api("/api/admin/users");
      if (!result.users.length) usersList.append(emptyMessage("No hay cuentas registradas."));
      for (const user of result.users) {
        const row = element("div", "dashboard-row");
        const main = element("div", "dashboard-row-main");
        main.append(element("div", "dashboard-row-title", user.name));
        main.append(element("div", "dashboard-row-meta", `${user.email} · ${user.role === "admin" ? "Administrador" : "Vendedor"}`));
        row.append(main);
        row.append(element("span", `status-pill ${user.role === "admin" ? "status-published" : ""}`, user.role === "admin" ? "Admin" : "Vendedor"));
        if (user.id !== currentUser.id) {
          row.append(button("Eliminar cuenta", "button-small button-danger", async () => {
            if (!window.confirm(`¿Eliminar la cuenta de ${user.name} y sus publicaciones?`)) return;
            try {
              await api(`/api/admin/users/${encodeURIComponent(user.id)}`, { method: "DELETE" });
              showNotice("Cuenta y publicaciones eliminadas.");
              await loadAdminPanel();
            } catch (error) {
              showNotice(error.message, true);
            }
          }));
        }
        usersList.append(row);
      }
    } catch (error) {
      usersList.append(emptyMessage(error.message));
    }
  }

  async function setCurrentUser(user) {
    currentUser = user;
    const signedIn = Boolean(user);
    document.querySelector("#open-login").classList.toggle("hidden", signedIn);
    document.querySelector("#open-register").classList.toggle("hidden", signedIn);
    document.querySelector("#account-menu").classList.toggle("hidden", !signedIn);
    document.querySelector("#account-name").textContent = signedIn ? user.name : "";
    document.querySelector(".seller-callout").classList.toggle("hidden", signedIn && user.role === "admin");
    workspace.classList.toggle("hidden", !signedIn);
    if (!signedIn) {
      workspaceContent.replaceChildren();
      return;
    }
    const isAdmin = user.role === "admin";
    document.querySelector("#workspace-eyebrow").textContent = isAdmin ? "ADMINISTRACIÓN" : "TU ESPACIO";
    document.querySelector("#workspace-title").textContent = isAdmin ? "Panel de administración" : "Panel de vendedor";
    if (isAdmin) await loadAdminPanel();
    else await loadSellerPanel();
  }

  document.querySelector("#auth-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const authForm = event.currentTarget;
    const error = document.querySelector("#auth-error");
    error.classList.add("hidden");
    const body = {
      email: document.querySelector("#email").value.trim(),
      password: document.querySelector("#password").value,
    };
    if (authMode === "register") {
      body.name = document.querySelector("#name").value.trim();
      body.phone = document.querySelector("#phone").value.trim();
    }
    try {
      if (authMode === "register") await api("/api/users", { method: "POST", body: JSON.stringify(body) });
      const result = await api("/api/sessions", { method: "POST", body: JSON.stringify({ email: body.email, password: body.password }) });
      sessionStorage.setItem(tokenKey, result.access_token);
      dialog.close();
      authForm.reset();
      await setCurrentUser(result.user);
      showNotice(authMode === "register" ? "Cuenta creada. Ya puedes publicar tu vehículo." : `Bienvenido, ${result.user.name}.`);
      workspace.scrollIntoView({ behavior: "smooth", block: "start" });
    } catch (submissionError) {
      error.textContent = submissionError.message;
      error.classList.remove("hidden");
    }
  });

  document.querySelector("#open-login").addEventListener("click", () => showAuth("login"));
  document.querySelector("#open-register").addEventListener("click", () => showAuth("register"));
  document.querySelector("#callout-register").addEventListener("click", () => {
    if (currentUser) workspace.scrollIntoView({ behavior: "smooth", block: "start" });
    else showAuth("register");
  });
  document.querySelector("#close-dialog").addEventListener("click", () => dialog.close());
  document.querySelector("#login-tab").addEventListener("click", () => showAuth("login"));
  document.querySelector("#register-tab").addEventListener("click", () => showAuth("register"));
  document.querySelector("#logout").addEventListener("click", () => {
    sessionStorage.removeItem(tokenKey);
    setCurrentUser(null);
    showNotice("Sesión cerrada.");
  });
  document.querySelector("#search-button").addEventListener("click", renderCatalog);
  document.querySelector("#clear-search").addEventListener("click", () => {
    searchInput.value = "";
    yearInput.value = "";
    renderCatalog();
  });
  searchInput.addEventListener("input", renderCatalog);
  yearInput.addEventListener("input", renderCatalog);

  (async () => {
    await loadCatalog();
    if (!token()) return;
    try {
      await setCurrentUser(await api("/api/me"));
    } catch (error) {
      if (error.status === 401) sessionStorage.removeItem(tokenKey);
      else showNotice(`No se pudo restaurar la sesión: ${error.message}`, true);
    }
  })();
})();
