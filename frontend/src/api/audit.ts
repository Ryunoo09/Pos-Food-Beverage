import { api } from './client'
import type { PaginatedResponse } from '../types/api.types'

/** Satu baris log audit. Nilai sebelum/sesudah berupa JSON bebas. */
export interface AuditLog {
  id: number
  occurred_at: string
  user_id?: string
  user_roles?: string[]
  action: string
  entity_type?: string
  entity_id?: string
  outlet_id?: string
  before_data?: unknown
  after_data?: unknown
  reason?: string
  ip_address?: string
  user_agent?: string
  request_id?: string
}

export interface AuditFilter {
  action?: string
  entity_type?: string
  user_id?: string
  from?: string
  to?: string
  page?: number
  per_page?: number
}

export const auditApi = {
  list: (params: AuditFilter) => api.get<PaginatedResponse<AuditLog>>('/audit-logs', { params }),
  /** Unduhan CSV: blob agar header Authorization tetap terkirim. */
  exportCsv: (params: AuditFilter) =>
    api.get<Blob>('/audit-logs/export', { params, responseType: 'blob' }),
}
