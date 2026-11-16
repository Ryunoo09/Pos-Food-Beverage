import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export const OPNAME_STATUSES = ['DRAFT', 'IN_PROGRESS', 'SUBMITTED', 'APPROVED', 'REJECTED', 'CANCELLED'] as const

export const OPNAME_STATUS_LABELS: Record<string, string> = {
  DRAFT: 'Draft',
  IN_PROGRESS: 'Berjalan',
  SUBMITTED: 'Menunggu persetujuan',
  APPROVED: 'Disetujui',
  REJECTED: 'Ditolak',
  CANCELLED: 'Dibatalkan',
}

export interface OpnameItem {
  id: string
  ingredient_id: string
  ingredient_code: string
  ingredient_name: string
  base_unit_code: string
  system_quantity: string
  counted_quantity?: string
  difference_quantity?: string
  difference_value?: string
  counted_at?: string
  reason?: string
}

export interface Opname {
  id: string
  code: string
  status: string
  scope: 'ALL' | 'SELECTED'
  note?: string
  rejected_reason?: string
  /** Pengaju sesi; dipakai UI untuk menyembunyikan Setujui dari pengaju sendiri. */
  submitted_by?: string
  started_at?: string
  submitted_at?: string
  approved_at?: string
  created_at: string
  items: OpnameItem[] | null
  total_items: number
  counted_items: number
}

export interface OpnameCreateInput {
  scope: 'ALL' | 'SELECTED'
  ingredient_ids: string[]
  note: string | null
}

export interface OpnameCountInput {
  items: { ingredient_id: string; counted_quantity: string; reason: string | null }[]
}

export const opnameApi = {
  create: (body: OpnameCreateInput) => api.post<ApiResponse<Opname>>('/stock-opnames', body),
  list: (params: { status?: string; page?: number; per_page?: number } = {}) =>
    api.get<PaginatedResponse<Opname>>('/stock-opnames', { params }),
  get: (id: string) => api.get<ApiResponse<Opname>>(`/stock-opnames/${id}`),
  start: (id: string) => api.post<ApiResponse<Opname>>(`/stock-opnames/${id}/start`),
  count: (id: string, body: OpnameCountInput) =>
    api.put<ApiResponse<Opname>>(`/stock-opnames/${id}/items`, body),
  submit: (id: string) => api.post<ApiResponse<Opname>>(`/stock-opnames/${id}/submit`),
  approve: (id: string) => api.post<ApiResponse<Opname>>(`/stock-opnames/${id}/approve`),
  reject: (id: string, reason: string) =>
    api.post<ApiResponse<Opname>>(`/stock-opnames/${id}/reject`, { reason }),
  cancel: (id: string) => api.post<ApiResponse<Opname>>(`/stock-opnames/${id}/cancel`),
  report: (id: string) => api.get<ApiResponse<Opname>>(`/stock-opnames/${id}/report`),
}
