import React from 'react'
import { Card, Chip, Ring } from '@/components/ui/Primitives'
import { formatBytes } from '@/lib/utils'
import { api } from '@/api/client'
import { ChevronRight, Plus } from 'lucide-react'

function AccountCard({ node, fileCount, onOpen }) {
  const account = node.Account || {}
  const quota = node.Quota || {}
  const used = quota.UsedBytes || 0
  const total = quota.TotalBytes || 0
  const pct = total > 0 ? (used / total) * 100 : 0
  const isPrimary = node.Role === 'owner'

  return (
    <Card
      as="button"
      interactive
      type="button"
      onClick={onOpen}
      title={`Open ${account.Email || 'this vault'}`}
      className="group p-4 flex items-center gap-3.5 text-left w-full min-h-[84px] cursor-pointer"
    >
      <Ring value={pct} size={48} thickness={5}>
        <span className="font-mono text-[10px] font-bold text-ink tabular-nums">
          {Math.round(pct)}%
        </span>
      </Ring>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="font-semibold text-sm text-ink truncate" title={account.Email}>
            {account.Email || 'Google Drive'}
          </p>
          {isPrimary && <Chip className="shrink-0">Primary</Chip>}
        </div>
        <p className="text-xs font-mono text-muted mt-1 tabular-nums truncate">
          {fileCount} file{fileCount === 1 ? '' : 's'} ·{' '}
          {formatBytes(Math.max(0, total - used))} free of {formatBytes(total)}
        </p>
      </div>

      <ChevronRight className="w-4 h-4 shrink-0 text-muted group-hover:text-brand-ink transition-colors" />
    </Card>
  )
}

/**
 * Where the pooled space comes from. Each account uses the same ring as the
 * headline total, smaller — the pool is visibly the sum of these — and each
 * card is the second way into that account's vault, alongside the rail.
 */
export function AccountsPanel({
  nodes = [],
  workspaceId,
  accountCounts = {},
  onOpenVault,
  onManage,
}) {
  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-4">
        <h2 className="section-title">Where your space comes from</h2>
        <button
          type="button"
          onClick={onManage}
          className="text-xs font-semibold text-brand-ink hover:underline underline-offset-4 cursor-pointer"
        >
          Manage
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
        {nodes.map((node) => (
          <AccountCard
            key={node.ID || node.Account?.ID}
            node={node}
            fileCount={accountCounts[node.Account?.ID] || 0}
            onOpen={() => onOpenVault(node.Account?.ID)}
          />
        ))}

        <button
          type="button"
          onClick={() => {
            window.location.href = api.connectSecondaryUrl(workspaceId)
          }}
          className="flex items-center justify-center gap-2 p-4 min-h-[84px] rounded-card border border-dashed border-line bg-transparent cursor-pointer text-sm font-semibold text-muted hover:text-brand-ink hover:border-brand-line hover:bg-brand-soft transition-colors duration-150"
        >
          <Plus className="w-4 h-4" />
          Add another account
        </button>
      </div>
    </section>
  )
}
