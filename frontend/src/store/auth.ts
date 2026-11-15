import { create } from 'zustand'
import type { AuthUser } from '../types/auth.types'

export type SessionStatus = 'idle' | 'loading' | 'authenticated' | 'anonymous'

interface AuthState {
  user: AuthUser | null
  accessToken: string | null
  permissions: string[]
  status: SessionStatus
  setAuth: (user: AuthUser, accessToken: string, permissions: string[]) => void
  setStatus: (status: SessionStatus) => void
  clearAuth: () => void
  hasPermission: (permission: string) => boolean
}

// Token sengaja hanya di memori (tanpa persist) agar tidak rentan XSS lewat localStorage.
export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  accessToken: null,
  permissions: [],
  status: 'idle',
  setAuth: (user, accessToken, permissions) =>
    set({ user, accessToken, permissions, status: 'authenticated' }),
  setStatus: (status) => set({ status }),
  clearAuth: () => set({ user: null, accessToken: null, permissions: [], status: 'anonymous' }),
  hasPermission: (permission) => get().permissions.includes(permission),
}))
