import React, { useEffect } from 'react'
import { IconButton } from './ui/Primitives'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * One shell for every overlay — centred dialog or right-hand drawer. Scrim,
 * escape handling, header and footer are identical everywhere, so the two
 * variants are the same object in a different position.
 */
export function Modal({
  isOpen,
  onClose,
  title,
  subtitle,
  icon: Icon,
  variant = 'dialog',
  width = 'max-w-lg',
  footer,
  children,
}) {
  useEffect(() => {
    if (!isOpen) return
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  }, [isOpen, onClose])

  if (!isOpen) return null
  const drawer = variant === 'drawer'

  return (
    <div
      className={cn(
        'fixed inset-0 z-50 flex p-4',
        drawer ? 'justify-end items-stretch' : 'items-center justify-center',
      )}
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div
        onClick={onClose}
        className="absolute inset-0 bg-[rgba(23,41,60,0.35)]"
        aria-hidden="true"
      />

      <div
        className={cn(
          'card relative w-full flex flex-col overflow-hidden shadow-[var(--shadow-overlay)] animate-fade-in',
          drawer ? 'max-w-md h-full' : `${width} max-h-[86vh]`,
        )}
      >
        <div className="flex items-center gap-3 px-5 h-16 shrink-0 border-b border-line">
          {Icon && (
            <span className="sunken w-9 h-9 grid place-items-center shrink-0">
              <Icon className="w-4 h-4 text-brand" />
            </span>
          )}
          <div className="min-w-0 flex-1">
            <h2 className="font-display text-base font-bold text-ink truncate">{title}</h2>
            {subtitle && <p className="text-xs text-muted truncate">{subtitle}</p>}
          </div>
          <IconButton label="Close" variant="ghost" onClick={onClose}>
            <X className="w-4 h-4" />
          </IconButton>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-5">{children}</div>

        {footer && (
          <div className="px-5 py-3.5 shrink-0 flex items-center justify-end gap-2 border-t border-line bg-sunken">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}
