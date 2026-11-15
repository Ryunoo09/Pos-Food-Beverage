import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../../store/auth'
import { canAccess } from '../../utils/permissions'

export interface PermissionGateProps {
  perm: string
  /** Bila diisi, pengguna tanpa permission diarahkan ke halaman ini. */
  redirect?: string
  fallback?: ReactNode
  children: ReactNode
}

export function PermissionGate({ perm, redirect, fallback = null, children }: PermissionGateProps) {
  const permissions = useAuthStore((state) => state.permissions)

  if (canAccess(permissions, perm)) return <>{children}</>
  if (redirect) return <Navigate to={redirect} replace />
  return <>{fallback}</>
}
