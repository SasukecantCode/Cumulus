import React from 'react'
import { KINDS } from '@/lib/files'
import { cn } from '@/lib/utils'

/**
 * A file's icon in a recessed tile. Every kind uses the brand blue — the
 * icon shape carries the type, not a colour, so a wall of files stays calm.
 */
export function FileGlyph({ kind, size = 'md' }) {
  const { icon: Icon } = KINDS[kind] || KINDS.other
  const box = size === 'sm' ? 'w-8 h-8' : 'w-10 h-10'
  const glyph = size === 'sm' ? 'w-4 h-4' : 'w-[18px] h-[18px]'

  return (
    <span className={cn('sunken grid place-items-center shrink-0', box)}>
      <Icon className={cn(glyph, 'text-brand')} />
    </span>
  )
}
