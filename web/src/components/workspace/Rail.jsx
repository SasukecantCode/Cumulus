import React from 'react'
import { CumulusLogo } from '@/components/CumulusLogo'
import { Button, Meter } from '@/components/ui/Primitives'
import { CATEGORIES } from '@/lib/files'
import { formatBytes, cn } from '@/lib/utils'
import { api } from '@/api/client'
import { ChevronRight, Cloud, LogOut, Mail, Plus, Users } from 'lucide-react'

/**
 * The left rail. Two questions, in order:
 *
 *   STORAGE     — whose files am I looking at? "Cumulus storage" is the whole
 *                 pool; expanding it lists each connected account, and picking
 *                 one narrows the workspace to that account's vault.
 *   FILE TYPES  — which kind of file, within whatever is selected above.
 *
 * The two are independent, so "Photos in emssasukeesports@gmail.com" is just
 * both selections at once.
 */
export function Rail({
  workspaceId,
  nodes = [],
  scope,
  onSelectScope,
  accountCounts = {},
  poolFileCount = 0,
  counts = {},
  selectedCategory,
  onSelectCategory,
  expanded,
  onToggleExpanded,
  totalCapacity = 0,
  totalUsed = 0,
  onManageAccounts,
  onSignOut,
}) {
  const usage = totalCapacity > 0 ? (totalUsed / totalCapacity) * 100 : 0

  return (
    <aside className="hidden lg:flex flex-col shrink-0 w-64 h-screen sticky top-0 bg-card border-r border-line">
      <div className="px-5 h-16 flex items-center border-b border-line shrink-0">
        <CumulusLogo size="md" />
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-3 py-4 flex flex-col gap-4">
        {/* ---- storage: the pool, then each vault inside it ---- */}
        <div>
          <span className="label px-2 pb-2">Storage</span>

          <div className="flex items-center gap-0.5">
            <button
              type="button"
              onClick={() => onSelectScope('all')}
              aria-current={scope === 'all' ? 'page' : undefined}
              className={cn(
                'flex-1 min-w-0 flex items-center gap-2.5 h-9 pl-2.5 pr-2 rounded-control cursor-pointer',
                'text-sm font-semibold transition-colors duration-150',
                scope === 'all'
                  ? 'bg-brand-soft text-brand-ink'
                  : 'text-muted hover:text-ink hover:bg-sunken',
              )}
            >
              <Cloud className="w-4 h-4 shrink-0" />
              <span className="flex-1 text-left truncate" title="Files uploaded through Cumulus">
                Cumulus storage
              </span>
              <span className="text-[11px] font-mono tabular-nums opacity-70">
                {poolFileCount}
              </span>
            </button>

            <button
              type="button"
              onClick={onToggleExpanded}
              aria-expanded={expanded}
              aria-label={expanded ? 'Collapse accounts' : 'Expand accounts'}
              className="w-7 h-9 grid place-items-center shrink-0 rounded-control text-muted hover:text-ink hover:bg-sunken cursor-pointer transition-colors duration-150"
            >
              <ChevronRight
                className={cn(
                  'w-4 h-4 transition-transform duration-150',
                  expanded && 'rotate-90',
                )}
              />
            </button>
          </div>

          {expanded && (
            <div className="mt-0.5 ml-4 pl-3 border-l border-line flex flex-col gap-0.5">
              {nodes.length === 0 && (
                <p className="px-2 py-2 text-xs text-muted">No accounts connected yet.</p>
              )}

              {nodes.map((node) => {
                const account = node.Account || {}
                const quota = node.Quota || {}
                const id = account.ID
                const selected = scope === id
                const email = account.Email || `Account #${id}`

                return (
                  <button
                    key={node.ID || id}
                    type="button"
                    onClick={() => onSelectScope(id)}
                    aria-current={selected ? 'page' : undefined}
                    title={`${email} — everything in this Drive`}
                    className={cn(
                      'flex items-center gap-2 min-h-9 py-1.5 px-2 rounded-control cursor-pointer',
                      'text-xs font-semibold transition-colors duration-150',
                      selected
                        ? 'bg-brand-soft text-brand-ink'
                        : 'text-muted hover:text-ink hover:bg-sunken',
                    )}
                  >
                    <Mail className="w-3.5 h-3.5 shrink-0" />
                    <span className="flex-1 min-w-0 text-left">
                      <span className="block truncate">{email.split('@')[0]}</span>
                      <span className="block truncate font-mono font-normal text-[10px] opacity-70">
                        {formatBytes(quota.UsedBytes || 0)} used
                      </span>
                    </span>
                    <span className="text-[11px] font-mono tabular-nums opacity-70">
                      {accountCounts[id] || 0}
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </div>

        {/* ---- file types, applied within whatever is selected above ---- */}
        <div>
          <span className="label px-2 pb-2">File types</span>
          <div className="flex flex-col gap-0.5">
            {CATEGORIES.map((cat) => {
              const Icon = cat.icon
              const selected = selectedCategory === cat.id
              const count = counts[cat.id] || 0

              return (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => onSelectCategory(cat.id)}
                  aria-current={selected ? 'page' : undefined}
                  className={cn(
                    'flex items-center gap-2.5 h-9 px-2.5 rounded-control cursor-pointer',
                    'text-sm font-semibold transition-colors duration-150',
                    selected
                      ? 'bg-brand-soft text-brand-ink'
                      : 'text-muted hover:text-ink hover:bg-sunken',
                  )}
                >
                  <Icon className="w-4 h-4 shrink-0" />
                  <span className="flex-1 text-left truncate">{cat.label}</span>
                  {count > 0 && (
                    <span className="text-[11px] font-mono tabular-nums opacity-70">{count}</span>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      </div>

      <div className="px-3 py-4 border-t border-line flex flex-col gap-3 shrink-0">
        <button
          type="button"
          onClick={onManageAccounts}
          className="flex items-center gap-2.5 h-9 px-2.5 rounded-control cursor-pointer text-sm font-semibold text-muted hover:text-ink hover:bg-sunken transition-colors duration-150"
        >
          <Users className="w-4 h-4 shrink-0" />
          <span className="flex-1 text-left">Accounts</span>
          <span className="chip chip-neutral">{nodes.length}</span>
        </button>

        <div className="sunken px-3 py-2.5 flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-2">
            <span className="label">Pooled</span>
            <span className="text-[11px] font-mono font-bold text-brand-ink tabular-nums">
              {Math.round(usage)}%
            </span>
          </div>
          <Meter value={usage} />
          <div className="flex justify-between text-[11px] font-mono text-muted tabular-nums">
            <span>{formatBytes(totalUsed)}</span>
            <span>{formatBytes(totalCapacity)}</span>
          </div>
        </div>

        <Button
          className="w-full"
          onClick={() => {
            window.location.href = api.connectSecondaryUrl(workspaceId)
          }}
        >
          <Plus className="w-4 h-4" />
          Add account
        </Button>

        <button
          type="button"
          onClick={onSignOut}
          className="flex items-center gap-2.5 h-9 px-2.5 rounded-control cursor-pointer text-sm font-semibold text-muted hover:text-danger hover:bg-[#fbecea] transition-colors duration-150"
        >
          <LogOut className="w-4 h-4 shrink-0" />
          <span className="flex-1 text-left">Sign out</span>
        </button>
      </div>
    </aside>
  )
}
