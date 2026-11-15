import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'

type Tone = 'green' | 'yellow' | 'red' | 'gray' | 'blue'

const TONES: Record<Tone, string> = {
  green: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-200',
  yellow: 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-100',
  red: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200',
  gray: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
  blue: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-200',
}

export function Badge({ tone = 'gray', children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap',
        TONES[tone],
      )}
    >
      {children}
    </span>
  )
}
