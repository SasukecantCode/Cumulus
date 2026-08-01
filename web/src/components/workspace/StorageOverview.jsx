import React from 'react'
import { Ring } from '@/components/ui/Primitives'
import { formatBytes, formatPercent, cn } from '@/lib/utils'

function Stat({ label, value, note, onClick }) {
  const Tag = onClick ? 'button' : 'div'
  return (
    <Tag
      type={onClick ? 'button' : undefined}
      onClick={onClick}
      className={cn(
        'sunken px-4 py-3 text-left transition-colors duration-150',
        onClick && 'cursor-pointer hover:border-brand-line hover:bg-brand-soft',
      )}
    >
      <span className="label">{label}</span>
      <p className="font-display text-xl font-bold text-ink mt-1 tabular-nums">{value}</p>
      {note && <p className="text-[11px] text-muted mt-0.5 truncate">{note}</p>}
    </Tag>
  )
}

/**
 * The headline band: one ring for whatever is selected, then the four numbers
 * that explain it. Identical shape for the whole pool and for a single vault
 * — only the fourth stat changes, so switching between them doesn't move
 * anything on screen.
 */
export function StorageOverview({
  totalCapacity = 0,
  totalUsed = 0,
  totalAvailable = 0,
  nodesCount = 0,
  totalFiles = 0,
  shareOfPool = 0,
  isVault = false,
  onManageAccounts,
}) {
  const usage = totalCapacity > 0 ? (totalUsed / totalCapacity) * 100 : 0

  return (
    <section className="card p-5 lg:p-6">
      <div className="flex flex-col sm:flex-row items-center gap-6 lg:gap-8">
        <div className="flex flex-col items-center gap-2 shrink-0">
          <Ring value={usage} size={132} thickness={10}>
            <span className="block font-display text-3xl font-bold text-ink tabular-nums">
              {Math.round(usage)}
              <span className="text-lg align-top">%</span>
            </span>
            <span className="block text-[11px] text-muted mt-1">in use</span>
          </Ring>
          <p className="text-xs text-muted">
            <span className="font-mono tabular-nums">{formatBytes(totalCapacity)}</span>{' '}
            {isVault ? 'on this account' : 'pooled'}
          </p>
        </div>

        <div className="flex-1 w-full grid grid-cols-2 xl:grid-cols-4 gap-3">
          <Stat
            label="Used"
            value={formatBytes(totalUsed)}
            note={isVault ? 'on this account' : `across ${nodesCount} account${nodesCount === 1 ? '' : 's'}`}
          />
          <Stat
            label="Free"
            value={formatBytes(totalAvailable)}
            note={totalAvailable > 0 ? 'room to spare' : 'connect another account'}
          />
          {isVault ? (
            <Stat
              label="Files"
              value={totalFiles}
              note="everything in this Drive"
            />
          ) : (
            <Stat
              label="Accounts"
              value={nodesCount}
              note="manage connections"
              onClick={onManageAccounts}
            />
          )}
          {isVault ? (
            <Stat
              label="Share of pool"
              value={formatPercent(shareOfPool)}
              note="of your total space"
            />
          ) : (
            <Stat label="Files" value={totalFiles} note="uploaded through Cumulus" />
          )}
        </div>
      </div>
    </section>
  )
}
