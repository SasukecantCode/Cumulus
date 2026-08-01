import React, { useMemo, useState } from 'react'
import { Button, Card, Chip, Notice, Segmented } from '@/components/ui/Primitives'
import { CloudMascot } from '@/components/CumulusLogo'
import { FileGlyph } from './FileGlyph'
import { CATEGORIES, KINDS, fileFields, googleNativeLabel, isCumulusUpload, kindOf } from '@/lib/files'
import { formatBytes, formatDate, cn } from '@/lib/utils'
import { api } from '@/api/client'
import { Check, Download, Info, LayoutGrid, Pencil, Rows3, Trash2, X } from 'lucide-react'

const SORTS = [
  { value: 'recent', label: 'Newest' },
  { value: 'name', label: 'Name' },
  { value: 'size', label: 'Largest' },
]

/* One column template, shared by the list header and every list row, so the
   two can never fall out of alignment. Inside a single vault the "Lives on"
   column would repeat the same address on every row, so it is dropped. */
const LIST_COLS = {
  true: 'md:grid-cols-[minmax(0,1fr)_100px_minmax(0,200px)_148px]',
  false: 'md:grid-cols-[minmax(0,1fr)_100px_148px]',
}

/**
 * The file browser: toolbar, then either a card grid or a dense list. Both
 * views share the same actions, rename field and empty state, so switching
 * view never changes what you can do.
 */
export function FileBrowser({
  files = [],
  workspaceId,
  accountMap = {},
  showAccountColumn = true,
  onRefresh,
  onSelectFileDetails,
  onUploadClick,
  hasVaults = false,
  selectedCategory = 'all',
  searchQuery = '',
}) {
  const cols = LIST_COLS[showAccountColumn]
  // showAccountColumn is only true at the pool, where every row is a Cumulus
  // upload; inside a vault the list mixes both origins and needs the badge.
  const isVault = !showAccountColumn
  const [view, setView] = useState('grid')
  const [sort, setSort] = useState('recent')
  const [renamingId, setRenamingId] = useState(null)
  const [draftName, setDraftName] = useState('')
  const [busyId, setBusyId] = useState(null)
  const [error, setError] = useState(null)

  const visible = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    return files
      .filter((file) => {
        const { name, mime } = fileFields(file)
        if (q && !name.toLowerCase().includes(q)) return false
        if (selectedCategory === 'all') return true
        return kindOf(mime, name) === selectedCategory
      })
      .sort((a, b) => {
        const fa = fileFields(a)
        const fb = fileFields(b)
        if (sort === 'name') return fa.name.localeCompare(fb.name)
        if (sort === 'size') return fb.size - fa.size
        return new Date(fb.createdAt || 0) - new Date(fa.createdAt || 0)
      })
  }, [files, searchQuery, selectedCategory, sort])

  const accountLabel = (accountId) =>
    accountMap[accountId] || (accountId ? `Account #${accountId}` : 'Cloud drive')

  const actions = {
    details: onSelectFileDetails,
    download: (f) => window.open(api.downloadFileUrl(workspaceId, f.id), '_blank'),
    rename: (f) => {
      setRenamingId(f.id)
      setDraftName(f.name)
      setError(null)
    },
    remove: async (f) => {
      if (!confirm(`Delete “${f.name}”? It is removed from the drive it lives on.`)) return
      setBusyId(f.id)
      try {
        await api.deleteFile(workspaceId, f.id)
        await onRefresh()
      } catch (err) {
        setError(err.message || 'Could not delete that file.')
      } finally {
        setBusyId(null)
      }
    },
  }

  const saveRename = async (id) => {
    if (!draftName.trim()) return setRenamingId(null)
    setBusyId(id)
    try {
      await api.renameFile(workspaceId, id, draftName.trim())
      setRenamingId(null)
      await onRefresh()
    } catch (err) {
      setError(err.message || 'Could not rename that file.')
    } finally {
      setBusyId(null)
    }
  }

  // Unfiltered reads as "All files"; the rail calls the same option "All
  // types" because there it sits next to the per-account tree.
  const heading =
    selectedCategory === 'all'
      ? 'All files'
      : CATEGORIES.find((c) => c.id === selectedCategory)?.label || 'All files'

  const renameProps = (id) => ({
    value: draftName,
    onChange: setDraftName,
    onSave: () => saveRename(id),
    onCancel: () => setRenamingId(null),
  })

  const rowProps = (file) => {
    const id = fileFields(file).id
    return {
      file,
      busy: busyId === id,
      renaming: renamingId === id,
      renameProps,
      accountLabel,
      actions,
      showAccount: showAccountColumn,
      showOrigin: isVault,
      cols,
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="min-w-0">
          <h2 className="section-title flex items-baseline gap-2">
            <span>{heading}</span>
            <span className="text-sm font-mono font-normal text-muted tabular-nums">
              {visible.length}
            </span>
          </h2>
          {/* The pool deliberately shows less than the vaults do, so say so
              rather than letting it read as missing files. */}
          <p className="text-xs text-muted mt-0.5">
            {isVault
              ? 'Everything in this account, including files that were already there.'
              : hasVaults
                ? 'Files you uploaded through Cumulus. Open an account to see everything in its Drive.'
                : 'Files you uploaded through Cumulus.'}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Segmented value={sort} onChange={setSort} options={SORTS} />
          <Segmented
            value={view}
            onChange={setView}
            options={[
              { value: 'grid', label: 'Grid view', icon: LayoutGrid },
              { value: 'list', label: 'List view', icon: Rows3 },
            ]}
          />
        </div>
      </div>

      {error && (
        <Notice onDismiss={() => setError(null)}>{error}</Notice>
      )}

      {visible.length === 0 ? (
        <EmptyState
          searching={Boolean(searchQuery)}
          isVault={isVault}
          hasVaults={hasVaults}
          onUploadClick={onUploadClick}
        />
      ) : view === 'grid' ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-3">
          {visible.map((file) => (
            <FileCard key={fileFields(file).id} {...rowProps(file)} />
          ))}
        </div>
      ) : (
        <Card className="p-2">
          <div
            className={cn(
              'hidden md:grid gap-4 items-center px-3 pb-2 mb-1 border-b border-line',
              cols,
            )}
          >
            <span className="label">Name</span>
            <span className="label">Size</span>
            {showAccountColumn && <span className="label">Lives on</span>}
            <span className="label text-right">Actions</span>
          </div>

          <div className="flex flex-col">
            {visible.map((file) => (
              <FileRow key={fileFields(file).id} {...rowProps(file)} />
            ))}
          </div>
        </Card>
      )}
    </section>
  )
}

/* ---------------------------------------------------------------- */

function FileCard({ file, busy, renaming, renameProps, accountLabel, actions, showAccount, showOrigin }) {
  const f = fileFields(file)
  const kind = kindOf(f.mime, f.name)
  const viaCumulus = isCumulusUpload(file)

  return (
    <Card
      interactive
      className={cn(
        'group p-4 flex flex-col gap-3',
        busy && 'opacity-60 pointer-events-none',
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <FileGlyph kind={kind} />
          {showOrigin && viaCumulus && <Chip className="shrink-0">Cumulus</Chip>}
        </div>
        <ActionCluster
          file={file}
          f={f}
          actions={actions}
          className="opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity duration-150"
        />
      </div>

      <div className="min-w-0">
        {renaming ? (
          <RenameField {...renameProps(f.id)} />
        ) : (
          <button
            type="button"
            onClick={() => actions.details(file)}
            title={f.name}
            className="block w-full text-left text-sm font-semibold text-ink truncate hover:text-brand-ink transition-colors cursor-pointer"
          >
            {f.name}
          </button>
        )}
        <p className="text-xs font-mono text-muted mt-1 tabular-nums">
          {googleNativeLabel(f.mime) || `${formatBytes(f.size)} · ${KINDS[kind].label}`}
        </p>
      </div>

      <div className="pt-2.5 border-t border-line flex items-center justify-between gap-2 text-[11px] text-muted">
        {/* inside a vault the account is a given, so the row shows when the
            file landed instead of repeating the same address on every card */}
        {showAccount ? (
          <>
            <span className="font-mono truncate" title={accountLabel(f.accountId)}>
              {accountLabel(f.accountId)}
            </span>
            {f.createdAt && (
              <span className="shrink-0">{formatDate(f.createdAt).split(',')[0]}</span>
            )}
          </>
        ) : (
          <span className="truncate">
            {f.createdAt ? `Uploaded ${formatDate(f.createdAt)}` : 'Upload date unknown'}
          </span>
        )}
      </div>
    </Card>
  )
}

function FileRow({ file, busy, renaming, renameProps, accountLabel, actions, showAccount, showOrigin, cols }) {
  const f = fileFields(file)
  const kind = kindOf(f.mime, f.name)
  const viaCumulus = isCumulusUpload(file)

  return (
    <div
      className={cn(
        'grid gap-4 items-center px-3 py-2 rounded-control',
        'hover:bg-sunken transition-colors duration-150',
        cols,
        busy && 'opacity-60 pointer-events-none',
      )}
    >
      <div className="flex items-center gap-3 min-w-0">
        <FileGlyph kind={kind} size="sm" />
        {renaming ? (
          <RenameField {...renameProps(f.id)} />
        ) : (
          <button
            type="button"
            onClick={() => actions.details(file)}
            className="min-w-0 text-left cursor-pointer"
            title={f.name}
          >
            <span className="flex items-center gap-2 min-w-0">
              <span className="text-sm font-semibold text-ink truncate hover:text-brand-ink transition-colors">
                {f.name}
              </span>
              {showOrigin && viaCumulus && <Chip className="shrink-0">Cumulus</Chip>}
            </span>
            <span className="block md:hidden text-[11px] font-mono text-muted mt-0.5 truncate">
              {formatBytes(f.size)}
              {showAccount && ` · ${accountLabel(f.accountId)}`}
            </span>
          </button>
        )}
      </div>

      <span className="hidden md:block text-xs font-mono text-muted tabular-nums truncate">
        {googleNativeLabel(f.mime) || formatBytes(f.size)}
      </span>
      {showAccount && (
        <span className="hidden md:block text-xs font-mono text-muted truncate">
          {accountLabel(f.accountId)}
        </span>
      )}

      <ActionCluster file={file} f={f} actions={actions} className="hidden md:flex justify-end" />
    </div>
  )
}

function ActionCluster({ file, f, actions, className }) {
  return (
    <div className={cn('flex items-center gap-0.5 shrink-0', className)}>
      <RowAction label="Details" onClick={() => actions.details(file)}>
        <Info className="w-4 h-4" />
      </RowAction>
      <RowAction label="Rename" onClick={() => actions.rename(f)}>
        <Pencil className="w-4 h-4" />
      </RowAction>
      <RowAction label="Download" onClick={() => actions.download(f)}>
        <Download className="w-4 h-4" />
      </RowAction>
      <RowAction label="Delete" danger onClick={() => actions.remove(f)}>
        <Trash2 className="w-4 h-4" />
      </RowAction>
    </div>
  )
}

function RowAction({ label, danger, children, ...rest }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className={cn(
        'w-8 h-8 grid place-items-center rounded-control text-muted cursor-pointer',
        'transition-colors duration-150',
        danger ? 'hover:text-danger hover:bg-[#fbecea]' : 'hover:text-brand-ink hover:bg-brand-soft',
      )}
      {...rest}
    >
      {children}
    </button>
  )
}

function RenameField({ value, onChange, onSave, onCancel }) {
  return (
    <div className="flex items-center gap-1 w-full min-w-0">
      <input
        autoFocus
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') onSave()
          if (e.key === 'Escape') onCancel()
        }}
        className="input flex-1 min-w-0 h-8 px-2.5 text-sm"
      />
      <RowAction label="Save name" onClick={onSave}>
        <Check className="w-4 h-4" />
      </RowAction>
      <RowAction label="Cancel" onClick={onCancel}>
        <X className="w-4 h-4" />
      </RowAction>
    </div>
  )
}

function EmptyState({ searching, isVault, hasVaults, onUploadClick }) {
  const heading = searching
    ? 'Nothing matches that'
    : isVault
      ? 'This account is empty'
      : 'Nothing uploaded yet'

  const body = searching
    ? 'Try a shorter search, or clear it to see everything you have stored.'
    : isVault
      ? 'This Drive has no files in it at all.'
      : hasVaults
        ? 'Upload a file and Cumulus tucks it into whichever connected drive has room. Files already in your Drives are listed under each account.'
        : 'Upload a file and Cumulus tucks it into whichever connected drive has room.'

  return (
    <Card className="px-6 py-14 flex flex-col items-center text-center gap-3">
      <CloudMascot size={88} />
      <h3 className="font-display text-base font-bold text-ink">{heading}</h3>
      <p className="text-sm text-muted max-w-sm leading-relaxed">{body}</p>
      {!searching && (
        <Button variant="primary" onClick={onUploadClick} className="mt-1">
          Upload your first file
        </Button>
      )}
    </Card>
  )
}
