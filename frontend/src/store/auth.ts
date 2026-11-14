import { create } from 'zustand'

export interface AuthUser {
  id: string
  username: string
  fullName: string
}

interface AuthState {
  user: AuthUser | null
  accessToken: string | null
  permissions: string[]
  setAuth: (user: AuthUser, accessToken: string, permissions: string[]) => void
  clearAuth: () => void
  hasPermission: (permission: string) => boolean
}

// Token sengaja hanya di memori (tanpa persist) agar tidak rentan XSS lewat localStorage.
export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  accessToken: null,
  permissions: [],
  setAuth: (user, accessToken, permissions) => set({ user, accessToken, permissions }),
  clearAuth: () => set({ user: null, accessToken: null, permissions: [] }),
  hasPermission: (permission) => get().permissions.includes(permission),
}))
