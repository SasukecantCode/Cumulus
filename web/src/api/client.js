const API_BASE = ''

async function request(url, options = {}) {
  const res = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers: {
      ...(options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...options.headers,
    },
  })

  if (options.raw) return res

  const data = await res.json()

  if (!res.ok) {
    const message = data?.error?.message || data?.message || 'Request failed'
    const code = data?.error?.code || data?.error || 'unknown'
    const err = new Error(message)
    err.code = code
    err.status = res.status
    throw err
  }

  return data
}

export const api = {
  // Workspace
  getWorkspaceNodes: (workspaceId) =>
    request(`/api/workspaces/${workspaceId}/nodes`),

  // Files
  listFiles: (workspaceId, params = {}) => {
    const query = new URLSearchParams(params).toString()
    return request(`/workspaces/${workspaceId}/files${query ? '?' + query : ''}`)
  },

  getFile: (workspaceId, fileId) =>
    request(`/workspaces/${workspaceId}/files/${fileId}`),

  // Adopts whatever is already sitting in the connected Drives, including
  // files that were there before Cumulus ever ran.
  syncWorkspace: (workspaceId) =>
    request(`/workspaces/${workspaceId}/sync`, { method: 'POST' }),

  uploadFile: (workspaceId, file, onProgress) => {
    const formData = new FormData()
    formData.append('file', file)
    return request(`/workspaces/${workspaceId}/files`, {
      method: 'POST',
      body: formData,
    })
  },

  deleteFile: (workspaceId, fileId) =>
    request(`/workspaces/${workspaceId}/files/${fileId}`, { method: 'DELETE' }),

  renameFile: (workspaceId, fileId, filename) =>
    request(`/workspaces/${workspaceId}/files/${fileId}`, {
      method: 'PATCH',
      body: JSON.stringify({ filename }),
    }),

  moveFile: (workspaceId, fileId, parentId) =>
    request(`/workspaces/${workspaceId}/files/${fileId}/move`, {
      method: 'PATCH',
      body: JSON.stringify({ parentId }),
    }),

  downloadFileUrl: (workspaceId, fileId) =>
    `${API_BASE}/workspaces/${workspaceId}/files/${fileId}/download`,

  // Auth
  googleLoginUrl: (workspaceId) =>
    workspaceId
      ? `${API_BASE}/auth/google?workspace_id=${workspaceId}`
      : `${API_BASE}/auth/google`,

  // Attaching another Google account goes through the same OAuth entry
  // point — passing the workspace makes the callback link, not create.
  connectSecondaryUrl: (workspaceId) =>
    `${API_BASE}/auth/google?workspace_id=${workspaceId || 1}`,

  // Account Management
  disconnectAccount: (workspaceId, cloudAccountId) =>
    request(`/workspaces/${workspaceId}/accounts/${cloudAccountId}`, { method: 'DELETE' }),
}
