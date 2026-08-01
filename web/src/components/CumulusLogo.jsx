import React from 'react'
import { cn } from '@/lib/utils'

const SIZES = { sm: 26, md: 32, lg: 64 }
const TEXT = { sm: 'text-[15px]', md: 'text-lg', lg: 'text-4xl sm:text-5xl' }

/**
 * The Cumulus mark: a flat cloud in the brand blue. Static — it is a logo,
 * not a toy.
 */
export function CumulusLogo({ size = 'md', layout = 'horizontal', withWordmark = true, className }) {
  const px = SIZES[size] || SIZES.md

  return (
    <div
      className={cn(
        'flex select-none',
        layout === 'vertical' ? 'flex-col items-center gap-3' : 'items-center gap-2.5',
        className,
      )}
    >
      <CloudMark size={px} />
      {withWordmark && (
        <span className={cn('font-display font-bold text-ink tracking-tight', TEXT[size] || TEXT.md)}>
          Cumulus
        </span>
      )}
    </div>
  )
}

const CLOUD_PATH =
  'M26 68C13 68 3 58.6 3 47c0-10.4 8-19 18.6-20.6C25.4 15.4 36 7.4 48.6 7.4c14.4 0 26.4 10.4 28.8 24C88.6 32.8 97 41 97 51.2 97 60.6 88.6 68 78 68H26Z'

function CloudMark({ size }) {
  return (
    <svg width={size} height={size * 0.72} viewBox="0 0 100 72" fill="none" aria-hidden="true">
      <path d={CLOUD_PATH} fill="var(--color-brand)" />
      <path
        d="M26 68h52c5 0 9.6-1.6 13-4.2-8.4 2-21.4 3-39 3-14 0-25-.8-32.4-2.2C23.4 66.8 24.6 68 26 68Z"
        fill="var(--color-brand-ink)"
        opacity="0.3"
      />
    </svg>
  )
}

/**
 * The larger cloud used in empty states. Outlined rather than filled, so it
 * reads as a placeholder instead of competing with the brand mark.
 */
export function CloudMascot({ size = 96, className }) {
  return (
    <svg
      width={size}
      height={size * 0.72}
      viewBox="0 0 100 72"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <path d={CLOUD_PATH} fill="var(--color-sunken)" stroke="var(--color-line)" strokeWidth="2" />
    </svg>
  )
}
