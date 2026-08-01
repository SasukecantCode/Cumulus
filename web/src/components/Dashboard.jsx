import React, { useMemo, useRef, useState } from 'react'
import { Rail } from './workspace/Rail'
import { PageHeader } from './workspace/PageHeader'
import { StorageOverview } from './workspace/StorageOverview'
import { AccountsPanel } from './workspace/AccountsPanel'
import { FileBrowser } from './workspace/FileBrowser'
import { AccountsDrawer } from './AccountsDrawer'
import { UploadModal } from './UploadModal'
import { FileDetailModal } from './FileDetailModal'
import { Notice } from './ui/Primitives'
import { CATEGORIES, fileFields, isCumulusUpload, kindOf } from '@/lib/files'
import { cn } from '@/lib/utils'
import { Cloud, CloudUpload, Loader2 } from 'lucide-react'

/**
 * The workspace.
 *
 * Two independent selections drive everything below the header:
 *   scope     — 'all' (the whole pool) or a cloud account ID (that vault)
 *   category  — 'all' or a file kind
 *
 * Scope decides which files exist as far as the rest of the page is
 * concerned, so counts, the storage readout and the browser all derive from
 * the same scoped list and can never disagree with each other.
 */
export function Dashboard({ workspaceContext }) {
  const {
    workspaceId,
    nodes,
    files,
    error,
    syncing,
    syncError,
    sync,
    signOut,
    accountMap,
    totalCapacity,
    totalUsed,
    totalAvailable,
    refresh,
  } = workspaceContext

  const [scope, setScope] = useState('all')
  const [treeOpen, setTreeOpen] = useState(true)
  const [selectedCategory, setSelectedCategory] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [accountsOpen, setAccountsOpen] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [droppedFiles, setDroppedFiles] = useState(null)
  const [detailFile, setDetailFile] = useState(null)
  const [refreshing, setRefreshing] = useState(false)
  const [dragging, setDragging] = useState(false)
  const dragDepth = useRef(0)

  /* An account that goes away (disconnected elsewhere) must not strand the
     view on an empty vault. */
  const activeNode = useMemo(
    () => (scope === 'all' ? null : nodes.find((n) => n.Account?.ID === scope) || null),
    [nodes, scope],
  )
  const effectiveScope = scope !== 'all' && !activeNode ? 'all' : scope

  /**
   * The two scopes answer different questions, so they show different sets:
   *
   *   Cumulus storage — what you put here, pooled across every account.
   *   a single account — everything in that Drive, Cumulus uploads and the
   *                      files that were already there alike.
   *
   * So the pool is not the sum of the vaults, and it is not meant to be.
   */
  const cumulusFiles = useMemo(() => files.filter(isCumulusUpload), [files])

  const scopedFiles = useMemo(
    () =>
      effectiveScope === 'all'
        ? cumulusFiles
        : files.filter((f) => fileFields(f).accountId === effectiveScope),
    [files, cumulusFiles, effectiveScope],
  )

  /* Kind counts follow the current scope; account counts never do, so the
     tree always shows how much each vault actually holds. */
  const counts = useMemo(() => {
    const tally = { all: scopedFiles.length }
    for (const file of scopedFiles) {
      const { name, mime } = fileFields(file)
      const kind = kindOf(mime, name)
      tally[kind] = (tally[kind] || 0) + 1
    }
    return tally
  }, [scopedFiles])

  const accountCounts = useMemo(() => {
    const tally = {}
    for (const file of files) {
      const { accountId } = fileFields(file)
      if (accountId != null) tally[accountId] = (tally[accountId] || 0) + 1
    }
    return tally
  }, [files])

  const quota = activeNode?.Quota || {}
  const view =
    effectiveScope === 'all'
      ? {
          title: 'Your drive',
          capacity: totalCapacity,
          used: totalUsed,
          available: totalAvailable,
          fileCount: cumulusFiles.length,
        }
      : {
          title: activeNode.Account?.Email || `Account #${effectiveScope}`,
          capacity: quota.TotalBytes || 0,
          used: quota.UsedBytes || 0,
          available: quota.AvailableBytes ?? Math.max(0, (quota.TotalBytes || 0) - (quota.UsedBytes || 0)),
          fileCount: scopedFiles.length,
        }

  const selectScope = (next) => {
    setScope(next)
    if (next !== 'all') setTreeOpen(true)
  }

  // Refresh means "go look at my Drives again", not just "re-read the
  // database" — otherwise files added in Drive would never turn up.
  const handleRefresh = async () => {
    setRefreshing(true)
    await sync()
    setRefreshing(false)
  }

  const openUpload = (initial = null) => {
    setDroppedFiles(initial)
    setUploadOpen(true)
  }

  /* drop files anywhere in the workspace */
  const onDragEnter = (e) => {
    if (!e.dataTransfer?.types?.includes('Files')) return
    dragDepth.current += 1
    setDragging(true)
  }
  const onDragLeave = () => {
    dragDepth.current = Math.max(0, dragDepth.current - 1)
    if (dragDepth.current === 0) setDragging(false)
  }
  const onDrop = (e) => {
    e.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    const dropped = Array.from(e.dataTransfer?.files || [])
    if (dropped.length) openUpload(dropped)
  }

  return (
    <div
      className="flex min-h-screen w-full"
      onDragEnter={onDragEnter}
      onDragOver={(e) => e.preventDefault()}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      <Rail
        workspaceId={workspaceId}
        nodes={nodes}
        scope={effectiveScope}
        onSelectScope={selectScope}
        accountCounts={accountCounts}
        poolFileCount={cumulusFiles.length}
        counts={counts}
        selectedCategory={selectedCategory}
        onSelectCategory={setSelectedCategory}
        expanded={treeOpen}
        onToggleExpanded={() => setTreeOpen((v) => !v)}
        totalCapacity={totalCapacity}
        totalUsed={totalUsed}
        onManageAccounts={() => setAccountsOpen(true)}
        onSignOut={signOut}
      />

      <div className="flex-1 min-w-0 flex flex-col">
        <PageHeader
          title={view.title}
          isVault={effectiveScope !== 'all'}
          onClearScope={() => selectScope('all')}
          fileCount={view.fileCount}
          totalUsed={view.used}
          totalCapacity={view.capacity}
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          onRefresh={handleRefresh}
          refreshing={refreshing || syncing}
          onUpload={() => openUpload(null)}
        />

        <main className="shell flex-1 py-6 flex flex-col gap-6">
          {error && <Notice>{error}</Notice>}
          {syncError && <Notice tone="info">{syncError}</Notice>}

          <StorageOverview
            totalCapacity={view.capacity}
            totalUsed={view.used}
            totalAvailable={view.available}
            nodesCount={nodes.length}
            totalFiles={view.fileCount}
            shareOfPool={totalCapacity > 0 ? (view.capacity / totalCapacity) * 100 : 0}
            isVault={effectiveScope !== 'all'}
            onManageAccounts={() => setAccountsOpen(true)}
          />

          {syncing && (
            <p className="text-xs text-muted inline-flex items-center gap-2">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
              Reading your connected Drives…
            </p>
          )}

          {effectiveScope === 'all' && (
            <AccountsPanel
              nodes={nodes}
              workspaceId={workspaceId}
              accountCounts={accountCounts}
              onOpenVault={selectScope}
              onManage={() => setAccountsOpen(true)}
            />
          )}

          {/* the rail is desktop-only; on narrow screens both selections ride here */}
          <div className="lg:hidden flex flex-col gap-2">
            <div className="-mx-5 px-5 flex items-center gap-2 overflow-x-auto pb-1">
              <button
                type="button"
                onClick={() => selectScope('all')}
                className={cn('btn shrink-0 h-8 px-3 text-xs', effectiveScope === 'all' && 'is-active')}
              >
                <Cloud className="w-3.5 h-3.5" />
                All accounts
              </button>
              {nodes.map((node) => {
                const id = node.Account?.ID
                const email = node.Account?.Email || `Account #${id}`
                return (
                  <button
                    key={node.ID || id}
                    type="button"
                    onClick={() => selectScope(id)}
                    className={cn(
                      'btn shrink-0 h-8 px-3 text-xs',
                      effectiveScope === id && 'is-active',
                    )}
                  >
                    {email.split('@')[0]}
                  </button>
                )
              })}
            </div>

            <div className="-mx-5 px-5 flex items-center gap-2 overflow-x-auto pb-1">
              {CATEGORIES.map((cat) => (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => setSelectedCategory(cat.id)}
                  className={cn(
                    'btn shrink-0 h-8 px-3 text-xs',
                    selectedCategory === cat.id && 'is-active',
                  )}
                >
                  {cat.label}
                </button>
              ))}
            </div>
          </div>

          <FileBrowser
            files={scopedFiles}
            workspaceId={workspaceId}
            accountMap={accountMap}
            showAccountColumn={effectiveScope === 'all'}
            onRefresh={refresh}
            onSelectFileDetails={setDetailFile}
            onUploadClick={() => openUpload(null)}
            hasVaults={nodes.length > 0}
            selectedCategory={selectedCategory}
            searchQuery={searchQuery}
          />
        </main>
      </div>

      {dragging && (
        <div className="fixed inset-0 z-50 grid place-items-center p-8 pointer-events-none bg-[rgba(23,41,60,0.28)]">
          <div className="card px-12 py-10 flex flex-col items-center gap-3 shadow-[var(--shadow-overlay)]">
            <CloudUpload className="w-9 h-9 text-brand" />
            <p className="font-display text-lg font-bold text-ink">Drop to upload</p>
            <p className="text-sm text-muted">They go to whichever drive has room.</p>
          </div>
        </div>
      )}

      <AccountsDrawer
        isOpen={accountsOpen}
        onClose={() => setAccountsOpen(false)}
        nodes={nodes}
        workspaceId={workspaceId}
        onRefresh={refresh}
        onSignOut={signOut}
      />

      <UploadModal
        isOpen={uploadOpen}
        onClose={() => setUploadOpen(false)}
        workspaceId={workspaceId}
        onUploadSuccess={refresh}
        initialFiles={droppedFiles}
      />

      <FileDetailModal
        file={detailFile}
        isOpen={Boolean(detailFile)}
        onClose={() => setDetailFile(null)}
        workspaceId={workspaceId}
        accountMap={accountMap}
      />
    </div>
  )
}
