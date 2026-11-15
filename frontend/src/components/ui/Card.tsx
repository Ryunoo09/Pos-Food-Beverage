import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <section
      className={cn(
        'rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900',
        className,
      )}
    >
      {children}
    </section>
  )
}

export function CardHeader({ title, action }: { title: ReactNode; action?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
      <h2 className="font-semibold text-slate-900 dark:text-slate-100">{title}</h2>
      {action}
    </header>
  )
}

export function CardBody({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn('p-4', className)}>{children}</div>
}

export function CardFooter({ children }: { children: ReactNode }) {
  return (
    <footer className="border-t border-slate-200 px-4 py-3 dark:border-slate-800">{children}</footer>
  )
}
