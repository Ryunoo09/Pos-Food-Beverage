import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'
import type { AuthPayload, AuthUser, Session } from '../types/auth.types'

export interface LoginBody {
  username: string
  password: string
}

export const authApi = {
  login: (body: LoginBody) => api.post<ApiResponse<AuthPayload>>('/auth/login', body),
  logout: () => api.post<void>('/auth/logout'),
  logoutAll: () => api.post<void>('/auth/logout-all'),
  me: () => api.get<ApiResponse<{ user: AuthUser; permissions: string[] }>>('/auth/me'),
  sseToken: () => api.post<ApiResponse<{ token: string; expires_in: number }>>('/auth/sse-token'),
  changePassword: (body: { current_password: string; new_password: string }) =>
    api.post<void>('/auth/change-password', body),
  sessions: () => api.get<ApiResponse<Session[]>>('/auth/sessions'),
  revokeSession: (id: string) => api.delete<void>(`/auth/sessions/${id}`),
  loginHistory: (params: { page?: number; per_page?: number } = {}) =>
    api.get<PaginatedResponse<LoginHistory>>('/auth/login-history', { params }),
}

export interface LoginHistory {
  id: string
  username_attempted: string
  success: boolean
  failure_reason?: string
  ip_address?: string
  user_agent?: string
  created_at: string
  full_name?: string
}
