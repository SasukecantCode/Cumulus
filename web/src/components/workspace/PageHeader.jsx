import React from 'react'
import { CumulusLogo } from '@/components/CumulusLogo'
import { Button, IconButton } from '@/components/ui/Primitives'
import { formatBytes } from '@/lib/utils'
import { ArrowLeft, RefreshCw, Search, Upload, X } from 'lucide-react'

/**
 * The workspace header: who you are looking at on the left, what you can do
 * about it on the right. Sticky, one row tall on desktop, and every control
 * in it is 36px so the row has a single baseline.
 */
export function PageHeader({
  title,
  isVault = false,
  onClearScope,
  fileCount,
  totalUsed,
  totalCapacity,
  searchQuery,
  setSearchQuery,
  onRefresh,
  refreshing,
  onUpload,
}) {
  return (
    <header className="sticky top-0 z-30 bg-card border-b border-line">
      <div className="shell py-3 flex flex-col gap-3 lg:h-16 lg:flex-row lg:items-center lg:justify-between lg:gap-6 lg:py-0">
        <div className="flex items-center gap-2.5 min-w-0">
          <span className="lg:hidden">
            <CumulusLogo size="sm" withWordmark={false} />
          </span>

          {/* One tap back to the whole pool whenever a single vault is open. */}
          {isVault && (
            <IconButton label="Back to all accounts" size="sm" onClick={onClearScope}>
              <ArrowLeft className="w-4 h-4" />
            </IconButton>
          )}

          <div className="min-w-0">
            <h1
              className="font-display text-lg font-bold text-ink leading-tight truncate"
              title={title}
            >
              {title}
            </h1>
            <p className="text-xs text-muted truncate">
              {isVault && <span className="font-semibold text-brand-ink">Account · </span>}
              {fileCount} file{fileCount === 1 ? '' : 's'}
              {!isVault && ' uploaded'} ·{' '}
              <span className="font-mono tabular-nums">
                {formatBytes(totalUsed)} of {formatBytes(totalCapacity)}
              </span>{' '}
              in use
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <div className="relative flex-1 lg:flex-none lg:w-72">
            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted pointer-events-none" />
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search files…"
              className="input h-9 pl-9 pr-9 text-sm"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                aria-label="Clear search"
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted hover:text-ink cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>

          <IconButton label="Refresh" onClick={onRefresh} disabled={refreshing}>
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          </IconButton>

          <Button variant="primary" onClick={onUpload}>
            <Upload className="w-4 h-4" />
            <span className="hidden sm:inline">Upload</span>
          </Button>
        </div>
      </div>
    </header>
  )
}
