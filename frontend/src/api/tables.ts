import { api } from './client'
import type { ApiResponse } from '../types/api.types'

export const TABLE_STATUSES = ['AVAILABLE', 'OCCUPIED', 'RESERVED', 'CLEANING'] as const
export type TableStatus = (typeof TABLE_STATUSES)[number]

export interface Table {
  id: string
  area?: string
  number: string
  capacity?: number
  status: TableStatus
  is_active: boolean
  sort_order: number
  created_at: string
}

export interface TableInput {
  area: string | null
  number: string
  capacity: number | null
  sort_order: number
  is_active?: boolean
}

export const tableApi = {
  list: (params: { area?: string; is_active?: boolean } = {}) =>
    api.get<ApiResponse<Table[]>>('/tables', { params }),
  get: (id: string) => api.get<ApiResponse<Table>>(`/tables/${id}`),
  create: (body: TableInput) => api.post<ApiResponse<Table>>('/tables', body),
  update: (id: string, body: TableInput) => api.patch<ApiResponse<Table>>(`/tables/${id}`, body),
  setStatus: (id: string, status: TableStatus) =>
    api.patch<void>(`/tables/${id}/status`, { status }),
  remove: (id: string) => api.delete<void>(`/tables/${id}`),
}
