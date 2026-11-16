import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface Shift {
  id: string
  user_id: string
  status: 'OPEN' | 'CLOSED'
  opened_at: string
  closed_at?: string
  opening_cash: string
  expected_cash?: string
  actual_cash?: string
  difference?: string
  difference_reason?: string
  version: number
}

export interface MethodTotal {
  method_code: string
  amount: string
  count: number
}

export interface ShiftSummary {
  shift: Shift
  cash_sales: string
  cash_refunds: string
  cash_in: string
  cash_out: string
  expected_cash: string
  by_method: MethodTotal[]
  voided_payment_count: number
  voided_payment_amount: string
  pending_payment_count: number
  difference: string
}

export interface CashMovement {
  id: string
  shift_id: string
  type: 'IN' | 'OUT'
  amount: string
  category?: string
  reason?: string
  created_by?: string
  created_at: string
}

export interface CloseInput {
  actual_cash: string
  difference_reason: string | null
}

export interface CashMovementInput {
  type: 'IN' | 'OUT'
  amount: string
  category: string | null
  reason: string
}

export const shiftApi = {
  list: (params: { status?: string; page?: number; per_page?: number } = {}) =>
    api.get<PaginatedResponse<Shift>>('/shifts', { params }),
  current: () => api.get<ApiResponse<Shift>>('/shifts/current'),
  open: (opening_cash: string) => api.post<ApiResponse<Shift>>('/shifts/open', { opening_cash }),
  summary: (id: string) => api.get<ApiResponse<ShiftSummary>>(`/shifts/${id}/summary`),
  close: (id: string, key: string, body: CloseInput) =>
    api.post<ApiResponse<ShiftSummary>>(`/shifts/${id}/close`, body, {
      headers: { 'Idempotency-Key': key },
    }),
  cashMovements: (id: string) =>
    api.get<ApiResponse<CashMovement[]>>(`/shifts/${id}/cash-movements`),
  addCashMovement: (id: string, body: CashMovementInput) =>
    api.post<ApiResponse<CashMovement>>(`/shifts/${id}/cash-movements`, body),
}
