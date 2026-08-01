import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react'
import { api } from '@/api/client'

const WorkspaceContext = createContext(null)

const WORKSPACE_KEY = 'cumulus.workspaceId'

/**
 * Which workspace is this browser looking at?
 *
 * OAuth hands the id back in the query string, but that only survives the
 * redirect — a later visit to "/" would silently fall back to workspace 1 and
 * show someone else's drive. Remembering the last one keeps the workspace
 * identity attached to the browser between visits.
 *
 * This is a client-side preference, not a credential. The API has no session
 * layer, so it is a convenience only.
 */
function initialWorkspaceId() {
  const fromUrl = parseInt(new URLSearchParams(window.location.search).get('workspace_id'))
  if (fromUrl) {
    try {
      localStorage.setItem(WORKSPACE_KEY, String(fromUrl))
    } catch {
      // private mode or storage disabled — the URL still carries us this session
    }
    return fromUrl
  }

  try {
    const remembered = parseInt(localStorage.getItem(WORKSPACE_KEY))
    if (remembered) return remembered
  } catch {
    // ignore and fall through to the default
  }
  return 1
}

export function WorkspaceProvider({ children }) {
  const [workspaceId, setWorkspaceId] = useState(initialWorkspaceId)
  const [workspace, setWorkspace] = useState(null)
  const [nodes, setNodes] = useState([])
  const [files, setFiles] = useState([])
  const [totalFiles, setTotalFiles] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [authenticated, setAuthenticated] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [syncError, setSyncError] = useState(null)
  const syncedOnceRef = useRef(false)

  const loadWorkspace = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.getWorkspaceNodes(workspaceId)
      setWorkspace(data.workspace)
      setNodes(data.nodes || [])
      setAuthenticated(true)
    } catch (err) {
      setError(err.message)
      setAuthenticated(false)
      setWorkspace(null)
      setNodes([])
    } finally {
      setLoading(false)
    }
  }, [workspaceId])

  /**
   * The API pages files (100 max per request, 50 if you don't ask), so a
   * single call would show at most 50 while the header reported the true
   * total. Walk every page and hold the whole list — the workspace filters,
   * counts and per-account vaults all read from it, so it has to be complete.
   */
  const loadFiles = useCallback(async () => {
    const PAGE = 100
    const MAX_PAGES = 200 // 20k files; a stop so a bad total can't spin forever

    try {
      const collected = []
      let total = 0

      for (let page = 0; page < MAX_PAGES; page++) {
        const data = await api.listFiles(workspaceId, { limit: PAGE, offset: page * PAGE })
        if (!data.success || !Array.isArray(data.data)) break

        collected.push(...data.data)
        total = data.pagination?.total ?? collected.length

        if (data.data.length < PAGE || collected.length >= total) break
      }

      setFiles(collected)
      setTotalFiles(Math.max(total, collected.length))
    } catch {
      // Silently fail for files — workspace might not have any
    }
  }, [workspaceId])

  const refresh = useCallback(async () => {
    await loadWorkspace()
    await loadFiles()
  }, [loadWorkspace, loadFiles])

  /**
   * Pull in whatever is already in the connected Drives, then reload. This is
   * what makes files that predate Cumulus appear. Sync failures are surfaced
   * but never block the reload — a partial listing still beats none.
   */
  const sync = useCallback(async () => {
    setSyncing(true)
    setSyncError(null)
    try {
      const res = await api.syncWorkspace(workspaceId)
      const failed = (res?.data?.accounts || []).filter((a) => a.error)
      if (failed.length) {
        setSyncError(
          `Could not read ${failed.map((a) => a.email || 'an account').join(', ')}. ` +
            'Everything else is up to date.',
        )
      }
    } catch (err) {
      setSyncError(err.message || 'Could not reach your Drives just now.')
    } finally {
      setSyncing(false)
    }
    await refresh()
  }, [workspaceId, refresh])

  /**
   * Sign out. The API keeps no session — there is no token or cookie to
   * revoke — so this drops the workspace this browser remembers and returns
   * to the landing page. The OAuth grant itself stays with Google; detaching
   * an account is what actually revokes access.
   */
  const signOut = useCallback(() => {
    try {
      localStorage.removeItem(WORKSPACE_KEY)
    } catch {
      // nothing to clear
    }
    // Drop workspace_id from the URL, or the next load would sign us back in.
    window.location.replace(window.location.pathname)
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  /* Adopt existing Drive contents once per session, as soon as we know the
     workspace is real. Without this the first visit looks empty for anyone
     who has not uploaded through Cumulus yet. */
  useEffect(() => {
    if (!authenticated || syncedOnceRef.current) return
    syncedOnceRef.current = true
    sync()
  }, [authenticated, sync])

  // Build a map of cloud account IDs to emails for display
  const accountMap = {}
  nodes.forEach((node) => {
    if (node.Account?.ID && node.Account?.Email) {
      accountMap[node.Account.ID] = node.Account.Email
    }
  })

  // Aggregate storage metrics
  const totalCapacity = nodes.reduce((sum, n) => sum + (n.Quota?.TotalBytes || 0), 0)
  const totalUsed = nodes.reduce((sum, n) => sum + (n.Quota?.UsedBytes || 0), 0)
  const totalAvailable = nodes.reduce((sum, n) => sum + (n.Quota?.AvailableBytes || 0), 0)

  const value = {
    workspaceId,
    setWorkspaceId,
    workspace,
    nodes,
    files,
    setFiles,
    totalFiles,
    loading,
    error,
    authenticated,
    syncing,
    syncError,
    sync,
    signOut,
    accountMap,
    totalCapacity,
    totalUsed,
    totalAvailable,
    refresh,
    loadFiles,
    loadWorkspace,
  }

  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  )
}

export function useWorkspace() {
  const ctx = useContext(WorkspaceContext)
  if (!ctx) throw new Error('useWorkspace must be used within WorkspaceProvider')
  return ctx
}
