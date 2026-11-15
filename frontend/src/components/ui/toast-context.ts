import { createContext } from 'react'

export type ToastTone = 'success' | 'error' | 'warning'

export interface ToastApi {
  push: (message: string, tone?: ToastTone) => void
}

export const ToastContext = createContext<ToastApi | null>(null)
