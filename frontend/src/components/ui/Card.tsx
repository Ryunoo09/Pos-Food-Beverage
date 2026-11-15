import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <section className={cn('rounded-card border border-line bg-surface', className)}>{children}</section>
  )
}

export function CardHeader({ title, action }: { title: ReactNode; action?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-rule px-5 py-3">
      <h2 className="text-base font-semibold">{title}</h2>
      {action}
    </header>
  )
}

export function CardBody({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn('p-5', className)}>{children}</div>
}

export function CardFooter({ children }: { children: ReactNode }) {
  return <footer className="border-t border-rule px-5 py-3">{children}</footer>
}
