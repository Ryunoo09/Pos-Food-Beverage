import { useContext } from 'react'
import { ToastContext } from '../components/ui/toast-context'
import type { ToastApi } from '../components/ui/toast-context'

export function useToast(): ToastApi {
  const api = useContext(ToastContext)
  if (!api) throw new Error('useToast harus dipakai di dalam <ToastProvider>.')
  return api
}
