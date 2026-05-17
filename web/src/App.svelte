<script>
  import { onMount } from 'svelte';
  import {
    createClient,
    createUser,
    deleteUser,
    deleteClient,
    getConfig,
    listClients,
    listClientTemplates,
    listUsers,
    resetUserPassword,
    rotateClientSecret,
    updateUser,
    updateClient
  } from './api.js';
  import { renderMarkdown } from './markdown.js';

  let activeTab = 'users';
  let config = { usersFile: '', configFile: '' };
  let users = [];
  let clients = [];
  let clientTemplates = [];
  let selectedTemplateId = '';
  let selectedTemplate = null;
  let loading = true;
  let error = '';
  let notice = '';
  let generatedClientSecret = null;
  let generatedUserPassword = null;
  let editingUsername = '';
  let editingClientId = '';
  let darkMode = false;

  let userForm = {
    username: '',
    displayName: '',
    email: '',
    groups: '',
    disabled: false
  };

  let clientForm = {
    clientId: '',
    clientName: '',
    public: false,
    redirectUris: '',
    scopes: 'openid, profile, email, groups',
    grantTypes: 'authorization_code',
    responseTypes: 'code',
    authorizationPolicy: 'two_factor',
    requirePkce: true,
    tokenEndpointAuthMethod: '',
    extra: {}
  };

  onMount(() => {
    initTheme();
    loadAll();
  });

  function initTheme() {
    const storedTheme = window.localStorage.getItem('theme');
    darkMode = storedTheme
      ? storedTheme === 'dark'
      : window.matchMedia('(prefers-color-scheme: dark)').matches;
    applyTheme();
  }

  function toggleTheme() {
    darkMode = !darkMode;
    window.localStorage.setItem('theme', darkMode ? 'dark' : 'light');
    applyTheme();
  }

  function applyTheme() {
    document.documentElement.dataset.theme = darkMode ? 'dark' : 'light';
  }

  async function loadAll() {
    loading = true;
    error = '';
    try {
      [config, users, clients, clientTemplates] = await Promise.all([
        getConfig(),
        listUsers(),
        listClients(),
        listClientTemplates()
      ]);
    } catch (err) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function submitUser() {
    error = '';
    notice = '';
    generatedUserPassword = null;
    try {
      const payload = userPayload();
      const result = editingUsername
        ? await updateUser(editingUsername, payload)
        : await createUser(payload);
      notice = `${editingUsername ? 'Updated' : 'Created'} user ${payload.username}`;
      if (result.generatedPassword) {
        generatedUserPassword = {
          username: result.username,
          password: result.generatedPassword
        };
      }
      resetUserForm();
      users = await listUsers();
    } catch (err) {
      error = err.message;
    }
  }

  async function resetPassword(user) {
    error = '';
    notice = '';
    generatedUserPassword = null;
    try {
      const material = await resetUserPassword(user.username);
      generatedUserPassword = {
        username: user.username,
        password: material.secret
      };
      notice = `Reset password for ${user.username}`;
    } catch (err) {
      error = err.message;
    }
  }

  async function removeUser(user) {
    if (!window.confirm(`Delete user ${user.username}?`)) {
      return;
    }
    error = '';
    notice = '';
    generatedUserPassword = null;
    try {
      await deleteUser(user.username);
      notice = `Deleted user ${user.username}`;
      if (editingUsername === user.username) {
        resetUserForm();
      }
      users = await listUsers();
    } catch (err) {
      error = err.message;
    }
  }

  function editUser(user) {
    editingUsername = user.username;
    generatedUserPassword = null;
    userForm = {
      username: user.username,
      displayName: user.displayName,
      email: user.email,
      groups: user.groups.join(', '),
      disabled: user.disabled
    };
  }

  function resetUserForm() {
    editingUsername = '';
    userForm = {
      username: '',
      displayName: '',
      email: '',
      groups: '',
      disabled: false
    };
  }

  async function submitClient() {
    error = '';
    notice = '';
    generatedClientSecret = null;
    try {
      const payload = clientPayload();
      const result = editingClientId
        ? await updateClient(editingClientId, payload)
        : await createClient(payload);
      notice = `${editingClientId ? 'Updated' : 'Created'} client ${payload.clientId}`;
      if (result.generatedClientSecret) {
        generatedClientSecret = {
          clientId: result.clientId,
          secret: result.generatedClientSecret
        };
      }
      resetClientForm();
      clients = await listClients();
    } catch (err) {
      error = err.message;
    }
  }

  async function rotateSecret(client) {
    error = '';
    notice = '';
    generatedClientSecret = null;
    try {
      const material = await rotateClientSecret(client.clientId);
      generatedClientSecret = {
        clientId: client.clientId,
        secret: material.secret
      };
      notice = `Rotated client secret for ${client.clientId}`;
    } catch (err) {
      error = err.message;
    }
  }

  async function removeClient(client) {
    if (!window.confirm(`Delete client ${client.clientId}?`)) {
      return;
    }
    error = '';
    notice = '';
    generatedClientSecret = null;
    try {
      await deleteClient(client.clientId);
      notice = `Deleted client ${client.clientId}`;
      if (editingClientId === client.clientId) {
        resetClientForm();
      }
      clients = await listClients();
    } catch (err) {
      error = err.message;
    }
  }

  function editClient(client) {
    editingClientId = client.clientId;
    generatedClientSecret = null;
    clientForm = {
      clientId: client.clientId,
      clientName: client.clientName,
      public: client.public,
      redirectUris: client.redirectUris.join(', '),
      scopes: client.scopes.join(', '),
      grantTypes: client.grantTypes.join(', '),
      responseTypes: client.responseTypes.join(', '),
      authorizationPolicy: client.authorizationPolicy || 'two_factor',
      requirePkce: client.requirePkce,
      tokenEndpointAuthMethod: client.tokenEndpointAuthMethod || '',
      extra: client.extra || {}
    };
    selectedTemplateId = '';
    selectedTemplate = null;
  }

  function resetClientForm() {
    editingClientId = '';
    clientForm = {
      clientId: '',
      clientName: '',
      public: false,
      redirectUris: '',
      scopes: 'openid, profile, email, groups',
      grantTypes: 'authorization_code',
      responseTypes: 'code',
      authorizationPolicy: 'two_factor',
      requirePkce: true,
      tokenEndpointAuthMethod: '',
      extra: {}
    };
    selectedTemplateId = '';
    selectedTemplate = null;
  }

  function clientPayload() {
    return {
      ...clientForm,
      redirectUris: splitList(clientForm.redirectUris),
      scopes: splitList(clientForm.scopes),
      grantTypes: splitList(clientForm.grantTypes),
      responseTypes: splitList(clientForm.responseTypes),
      extra: clientForm.extra || {}
    };
  }

  function applyClientTemplate() {
    const template = clientTemplates.find((item) => item.id === selectedTemplateId);
    selectedTemplate = template || null;
    if (!template) {
      return;
    }
    const client = template.client;
    generatedClientSecret = null;
    editingClientId = '';
    clientForm = {
      clientId: client.clientId || '',
      clientName: client.clientName || template.title,
      public: Boolean(client.public),
      redirectUris: (client.redirectUris || []).join(', '),
      scopes: (client.scopes || []).join(', '),
      grantTypes: (client.grantTypes || []).join(', '),
      responseTypes: (client.responseTypes || []).join(', '),
      authorizationPolicy: client.authorizationPolicy || 'two_factor',
      requirePkce: Boolean(client.requirePkce),
      tokenEndpointAuthMethod: client.tokenEndpointAuthMethod || '',
      extra: client.extra || {}
    };
  }

  function userPayload() {
    return {
      ...userForm,
      groups: splitList(userForm.groups)
    };
  }

  function splitList(value) {
    return String(value)
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
</script>

<main>
  <header class="topbar">
    <div>
      <h1>Authelia WebUI</h1>
      <p>{config.usersFile || '/config/users_database.yml'} · {config.configFile || '/config/configuration.yml'} · {config.autheliaBinary || 'authelia'}</p>
    </div>
    <div class="top-actions">
      <button class="secondary" type="button" on:click={toggleTheme}>{darkMode ? 'Light mode' : 'Dark mode'}</button>
      <button class="secondary" type="button" on:click={loadAll}>Refresh</button>
    </div>
  </header>

  <section class="tabs" aria-label="Sections">
    <button class:active={activeTab === 'users'} type="button" on:click={() => (activeTab = 'users')}>Users</button>
    <button class:active={activeTab === 'clients'} type="button" on:click={() => (activeTab = 'clients')}>Clients</button>
  </section>

  {#if error}
    <div class="alert error">{error}</div>
  {/if}
  {#if notice}
    <div class="alert success">{notice}</div>
  {/if}

  {#if loading}
    <div class="empty">Loading</div>
  {:else if activeTab === 'users'}
    <section class="layout">
      <form class="panel" on:submit|preventDefault={submitUser}>
        <div class="panel-title">
          <h2>{editingUsername ? 'Edit User' : 'Create User'}</h2>
          {#if editingUsername}
            <button class="secondary compact" type="button" on:click={resetUserForm}>Cancel</button>
          {/if}
        </div>
        <label>
          Username
          <input bind:value={userForm.username} autocomplete="off" disabled={Boolean(editingUsername)} required />
        </label>
        <label>
          Display name
          <input bind:value={userForm.displayName} autocomplete="name" />
        </label>
        <label>
          Email
          <input bind:value={userForm.email} type="email" autocomplete="email" required />
        </label>
        <label>
          Groups
          <input bind:value={userForm.groups} placeholder="admins, dev" />
        </label>
        <label class="check">
          <input bind:checked={userForm.disabled} type="checkbox" />
          Disabled
        </label>
        {#if generatedUserPassword}
          <div class="secret-box">
            <span>Password for {generatedUserPassword.username}</span>
            <code>{generatedUserPassword.password}</code>
          </div>
        {/if}
        <button type="submit">{editingUsername ? 'Save user' : 'Create user'}</button>
      </form>

      <section class="panel list">
        <h2>Users</h2>
        {#if users.length === 0}
          <div class="empty">No users found</div>
        {:else}
          {#each users as user}
            <article class="row">
              <div>
                <strong>{user.username}</strong>
                <span>{user.displayName || user.email}</span>
              </div>
              <div class="meta">
                <span>{user.email}</span>
                <span>{user.groups.length ? user.groups.join(', ') : 'no groups'}</span>
                {#if user.disabled}<span class="badge">disabled</span>{/if}
              </div>
              <div class="actions">
                <button class="secondary compact" type="button" on:click={() => editUser(user)}>Edit</button>
                <button class="secondary compact" type="button" on:click={() => resetPassword(user)}>Reset password</button>
                <button class="danger compact" type="button" on:click={() => removeUser(user)}>Delete</button>
              </div>
            </article>
          {/each}
        {/if}
      </section>
    </section>
  {:else}
    <section class="layout client-layout">
      <form class="panel" on:submit|preventDefault={submitClient}>
        <div class="panel-title">
          <h2>{editingClientId ? 'Edit OIDC Client' : 'Create OIDC Client'}</h2>
          {#if editingClientId}
            <button class="secondary compact" type="button" on:click={resetClientForm}>Cancel</button>
          {/if}
        </div>
        {#if !editingClientId && clientTemplates.length > 0}
          <label>
            Template
            <select bind:value={selectedTemplateId} on:change={applyClientTemplate}>
              <option value="">Generic</option>
              {#each clientTemplates as template}
                <option value={template.id}>{template.title}</option>
              {/each}
            </select>
          </label>
        {/if}
        <label>
          Client ID
          <input bind:value={clientForm.clientId} autocomplete="off" disabled={Boolean(editingClientId)} required />
        </label>
        <label>
          Client name
          <input bind:value={clientForm.clientName} required />
        </label>
        <label>
          Redirect URIs
          <textarea bind:value={clientForm.redirectUris} placeholder="https://app.example.com/oauth2/callback" required></textarea>
        </label>
        <label class="check">
          <input bind:checked={clientForm.public} type="checkbox" />
          Public client
        </label>
        {#if generatedClientSecret}
          <div class="secret-box">
            <span>Client secret for {generatedClientSecret.clientId}</span>
            <code>{generatedClientSecret.secret}</code>
          </div>
        {/if}
        <label>
          Scopes
          <input bind:value={clientForm.scopes} />
        </label>
        <label>
          Grant types
          <input bind:value={clientForm.grantTypes} />
        </label>
        <label>
          Response types
          <input bind:value={clientForm.responseTypes} />
        </label>
        <label>
          Authorization policy
          <select bind:value={clientForm.authorizationPolicy}>
            <option value="two_factor">two_factor</option>
            <option value="one_factor">one_factor</option>
            <option value="deny">deny</option>
          </select>
        </label>
        <label class="check">
          <input bind:checked={clientForm.requirePkce} type="checkbox" />
          Require PKCE
        </label>
        {#if selectedTemplate?.applicationMarkdown}
          <section class="instructions">
            <h3>Application Setup</h3>
            <div class="markdown-body">
              {@html renderMarkdown(selectedTemplate.applicationMarkdown)}
            </div>
          </section>
        {/if}
        <button type="submit">{editingClientId ? 'Save client' : 'Create client'}</button>
      </form>

      <section class="panel list">
        <h2>Clients</h2>
        {#if clients.length === 0}
          <div class="empty">No clients found</div>
        {:else}
          {#each clients as client}
            <article class="row">
              <div>
                <strong>{client.clientId}</strong>
                <span>{client.clientName}</span>
              </div>
              <div class="meta">
                <span>{client.redirectUris.join(', ')}</span>
                <span>{client.scopes.join(', ')}</span>
                <span class="badge">{client.public ? 'public' : 'confidential'}</span>
              </div>
              <div class="actions">
                <button class="secondary compact" type="button" on:click={() => editClient(client)}>Edit</button>
                {#if !client.public}
                  <button class="secondary compact" type="button" on:click={() => rotateSecret(client)}>Rotate secret</button>
                {/if}
                <button class="danger compact" type="button" on:click={() => removeClient(client)}>Delete</button>
              </div>
            </article>
          {/each}
        {/if}
      </section>
    </section>
  {/if}
</main>
