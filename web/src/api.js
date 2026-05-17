async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    },
    ...options
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || `Request failed with ${response.status}`);
  }
  return body;
}

export function getConfig() {
  return request('/api/config');
}

export function listUsers() {
  return request('/api/users');
}

export function createUser(payload) {
  return request('/api/users', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export function updateUser(username, payload) {
  return request(`/api/users/${encodeURIComponent(username)}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteUser(username) {
  const response = await fetch(`/api/users/${encodeURIComponent(username)}`, {
    method: 'DELETE'
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `Request failed with ${response.status}`);
  }
}

export function resetUserPassword(username) {
  return request(`/api/users/${encodeURIComponent(username)}/password`, {
    method: 'POST',
    body: JSON.stringify({})
  });
}

export function listClients() {
  return request('/api/clients');
}

export function listClientTemplates() {
  return request('/api/client-templates');
}

export function createClient(payload) {
  return request('/api/clients', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export function updateClient(clientId, payload) {
  return request(`/api/clients/${encodeURIComponent(clientId)}`, {
    method: 'PUT',
    body: JSON.stringify(payload)
  });
}

export async function deleteClient(clientId) {
  const response = await fetch(`/api/clients/${encodeURIComponent(clientId)}`, {
    method: 'DELETE'
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `Request failed with ${response.status}`);
  }
}

export function rotateClientSecret(clientId) {
  return request(`/api/clients/${encodeURIComponent(clientId)}/secret`, {
    method: 'POST',
    body: JSON.stringify({})
  });
}
