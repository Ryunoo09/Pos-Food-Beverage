import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'
import { ToastContext } from './toast-context'
import type { ToastTone } from './toast-context'

interface ToastItem {
  id: number
  message: string
  tone: ToastTone
}

const TONES: Record<ToastTone, string> = {
  success: 'border-positive bg-pill-positive text-pill-positive-ink',
  error: 'border-negative bg-pill-negative text-pill-negative-ink',
  warning: 'border-warning bg-pill-warning text-pill-warning-ink',
}

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const timers = useRef<number[]>([])

  const push = useCallback((message: string, tone: ToastTone = 'success') => {
    const id = ++nextId
    setItems((prev) => [...prev, { id, message, tone }])
    timers.current.push(
      window.setTimeout(() => setItems((prev) => prev.filter((item) => item.id !== id)), 4000),
    )
  }, [])

  useEffect(() => () => timers.current.forEach((timer) => window.clearTimeout(timer)), [])

  const api = useMemo(() => ({ push }), [push])

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-2"
      >
        {items.map((item) => (
          <div
            key={item.id}
            role="status"
            className={cn('rounded-control border-l-4 px-4 py-3 text-sm shadow-lg', TONES[item.tone])}
          >
            {item.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
