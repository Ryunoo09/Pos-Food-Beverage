import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'

type Tone = 'green' | 'yellow' | 'red' | 'gray' | 'blue'

const TONES: Record<Tone, string> = {
  green: 'bg-pill-positive text-pill-positive-ink',
  yellow: 'bg-pill-warning text-pill-warning-ink',
  red: 'bg-pill-negative text-pill-negative-ink',
  gray: 'bg-pill-neutral text-pill-neutral-ink',
  blue: 'bg-pill-info text-pill-info-ink',
}

export function Badge({ tone = 'gray', children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span
      className={cn(
        'inline-flex flex-none items-center rounded-chip px-2 py-0.5 text-[11px] font-medium whitespace-nowrap',
        TONES[tone],
      )}
    >
      {children}
    </span>
  )
}
