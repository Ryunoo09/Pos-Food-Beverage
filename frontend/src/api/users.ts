import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface User {
  id: string
  username: string
  full_name: string
  email?: string
  is_active: boolean
  must_change_password: boolean
  roles: string[]
}

export interface Role {
  id: string
  code: string
  name: string
  is_system: boolean
  permissions?: string[]
}

export interface CreateUserInput {
  username: string
  full_name: string
  email: string | null
  password: string
  role_ids: string[]
}

export const usersApi = {
  list: (params: { q?: string; role?: string; is_active?: boolean; page?: number; per_page?: number }) =>
    api.get<PaginatedResponse<User>>('/users', { params }),
  create: (body: CreateUserInput) => api.post<ApiResponse<User>>('/users', body),
  activate: (id: string) => api.post<ApiResponse<User>>(`/users/${id}/activate`),
  deactivate: (id: string) => api.post<ApiResponse<User>>(`/users/${id}/deactivate`),
  resetPassword: (id: string, password: string) =>
    api.post<ApiResponse<unknown>>(`/users/${id}/reset-password`, { password }),
}

export const rolesApi = {
  list: () => api.get<ApiResponse<Role[]>>('/roles'),
}
