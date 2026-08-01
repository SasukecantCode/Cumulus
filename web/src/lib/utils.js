import clsx from 'clsx'

export function cn(...inputs) {
  return clsx(inputs)
}

export function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

/**
 * Percentages for display. A 15 GB account inside a 4.6 TB pool is a real
 * share but rounds to zero, and "0%" reads as "nothing" — so anything above
 * zero keeps at least one significant step.
 */
export function formatPercent(value) {
  if (!Number.isFinite(value) || value <= 0) return '0%'
  if (value < 1) return '<1%'
  return `${Math.round(value)}%`
}

export function formatDate(dateStr) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
