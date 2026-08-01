import React from 'react'
import { cn } from '@/lib/utils'

/**
 * The shared vocabulary. Every screen is assembled from these, so a card
 * in the workspace, a card on the landing page and a card in a modal are
 * literally the same shape, and every control lands on the same height.
 *
 * Control heights: sm 32px · md 36px · lg 44px. Inputs match md.
 */

const HEIGHTS = {
  sm: 'h-8 px-3 text-xs gap-1.5',
  md: 'h-9 px-3.5 text-sm gap-2',
  lg: 'h-11 px-5 text-[15px] gap-2.5',
}

export function Card({ as: Tag = 'div', interactive = false, className, children, ...rest }) {
  return (
    <Tag className={cn('card', interactive && 'card-interactive', className)} {...rest}>
      {children}
    </Tag>
  )
}

export function Button({
  variant = 'default',
  size = 'md',
  active = false,
  className,
  children,
  ...rest
}) {
  return (
    <button
      type="button"
      className={cn(
        'btn',
        HEIGHTS[size],
        variant === 'primary' && 'btn-primary',
        variant === 'ghost' && 'btn-ghost',
        variant === 'danger' && 'btn-danger',
        active && 'is-active',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  )
}

/** A square button holding a single icon — same height as its text siblings. */
export function IconButton({ label, size = 'md', variant = 'default', active = false, className, children, ...rest }) {
  const box = { sm: 'w-8 h-8', md: 'w-9 h-9', lg: 'w-11 h-11' }[size]
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className={cn(
        'btn shrink-0 p-0',
        box,
        variant === 'ghost' && 'btn-ghost',
        variant === 'danger' && 'btn-danger',
        active && 'is-active',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  )
}

export function Meter({ value = 0, className }) {
  const pct = Math.min(100, Math.max(0, value))
  return (
    <div
      className={cn('meter', className)}
      role="progressbar"
      aria-valuenow={Math.round(pct)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div className="meter-fill" style={{ width: `${pct}%` }} />
    </div>
  )
}

export function Chip({ neutral = false, className, children, ...rest }) {
  return (
    <span className={cn('chip', neutral && 'chip-neutral', className)} {...rest}>
      {children}
    </span>
  )
}

/**
 * Segmented control. One bordered track, options fill it; the selected one
 * gets the brand tint. Used for both sort and view so they read as a pair.
 */
export function Segmented({ options, value, onChange, className }) {
  return (
    <div
      className={cn(
        'inline-flex items-center gap-1 h-9 p-1 rounded-control border border-line bg-card',
        className,
      )}
    >
      {options.map((opt) => {
        const Icon = opt.icon
        const selected = value === opt.value
        const iconOnly = Boolean(Icon)
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            title={opt.label}
            aria-label={opt.label}
            aria-pressed={selected}
            className={cn(
              'inline-flex items-center justify-center rounded-[6px] text-xs font-semibold cursor-pointer',
              'transition-colors duration-150',
              iconOnly ? 'w-7 h-7' : 'h-7 px-3',
              selected
                ? 'bg-brand-soft text-brand-ink'
                : 'text-muted hover:text-ink hover:bg-sunken',
            )}
          >
            {Icon ? <Icon className="w-4 h-4" /> : opt.label}
          </button>
        )
      })}
    </div>
  )
}

/**
 * A capacity ring: brand arc over a neutral track. Same component at every
 * size, so the pooled total and a single account are visibly the same idea.
 */
export function Ring({ value = 0, size = 132, thickness = 10, children }) {
  const pct = Math.min(100, Math.max(0, value))
  const r = (size - thickness) / 2
  const circumference = 2 * Math.PI * r

  return (
    <div
      className="relative shrink-0 grid place-items-center"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} className="absolute inset-0 -rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--color-line)"
          strokeWidth={thickness}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--color-brand)"
          strokeWidth={thickness}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - pct / 100)}
          style={{ transition: 'stroke-dashoffset 0.4s ease' }}
        />
      </svg>
      <div className="relative text-center leading-none">{children}</div>
    </div>
  )
}

/** Shared empty / error framing so every "nothing here" looks the same. */
export function Notice({ tone = 'error', className, children, onDismiss }) {
  return (
    <div
      className={cn(
        'flex items-center justify-between gap-3 px-4 py-3 rounded-control border text-sm',
        tone === 'error'
          ? 'border-[#f0cec8] bg-[#fbecea] text-danger'
          : 'border-brand-line bg-brand-soft text-brand-ink',
        className,
      )}
      role={tone === 'error' ? 'alert' : undefined}
    >
      <span className="min-w-0">{children}</span>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          aria-label="Dismiss"
          className="shrink-0 opacity-70 hover:opacity-100 cursor-pointer"
        >
          ✕
        </button>
      )}
    </div>
  )
}
