import { api } from './client'
import type { ApiResponse } from '../types/api.types'

export interface OutletSettings {
  tax_inclusive: boolean
  tax_on_service: boolean
  rounding_mode: string
  discount_cashier_max_percent: string
  discount_approval_threshold_amount: string
  refund_approval_threshold: string
}

export interface TaxConfig {
  id: string
  type: string
  name: string
  rate_percent: string
  applies_to: string[]
  is_active: boolean
  effective_from: string
}

export interface Discount {
  id: string
  code: string
  name: string
  discount_type: string
  value: string
  scope: string
  min_spend: string
  max_discount?: string
  starts_at?: string
  ends_at?: string
  days_of_week?: number[]
  requires_approval: boolean
  is_active: boolean
}

export interface DiscountInput {
  code: string
  name: string
  discount_type: string
  value: string
  scope: string
  min_spend: string
  max_discount: string | null
  starts_at: string | null
  ends_at: string | null
  days_of_week: number[] | null
  requires_approval: boolean
}

// Daftar ini harus sama dengan switch validasi di settings.Service.UpdateSettings;
// nilai di luar daftar ditolak backend dengan VALIDATION_ERROR.
export const ROUNDING_MODES = [
  { value: 'NONE', label: 'Tanpa pembulatan' },
  { value: 'NEAREST_100', label: 'Ke Rp100 terdekat' },
  { value: 'NEAREST_500', label: 'Ke Rp500 terdekat' },
  { value: 'UP_100', label: 'Bulatkan ke atas Rp100' },
] as const

export const TAX_TYPES = [
  { value: 'TAX', label: 'Pajak' },
  { value: 'SERVICE', label: 'Service charge' },
] as const

export const DISCOUNT_TYPES = [
  { value: 'PERCENT', label: 'Persen' },
  { value: 'AMOUNT', label: 'Nominal' },
] as const

// Backend hanya menerima ITEM atau ORDER (lihat discount.validate).
export const DISCOUNT_SCOPES = [
  { value: 'ORDER', label: 'Seluruh pesanan' },
  { value: 'ITEM', label: 'Per item' },
] as const

export const SERVICE_TYPES = [
  { value: 'DINE_IN', label: 'Dine-in' },
  { value: 'TAKEAWAY', label: 'Takeaway' },
  { value: 'DELIVERY', label: 'Delivery' },
] as const

export const DAY_LABELS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'] as const

export const settingsApi = {
  outlet: {
    get: () => api.get<ApiResponse<OutletSettings>>('/settings/outlet'),
    update: (body: Partial<OutletSettings>) =>
      api.patch<ApiResponse<OutletSettings>>('/settings/outlet', body),
  },
  taxes: {
    list: () => api.get<ApiResponse<TaxConfig[]>>('/settings/taxes'),
    create: (body: {
      type: string
      name: string
      rate_percent: string
      applies_to: string[]
      effective_from?: string | null
    }) => api.post<ApiResponse<TaxConfig>>('/settings/taxes', body),
    activate: (id: string) =>
      api.post<ApiResponse<TaxConfig>>(`/settings/taxes/${id}/activate`),
    deactivate: (id: string) =>
      api.post<ApiResponse<TaxConfig>>(`/settings/taxes/${id}/deactivate`),
  },
  discounts: {
    list: () => api.get<ApiResponse<Discount[]>>('/discounts'),
    get: (id: string) => api.get<ApiResponse<Discount>>(`/discounts/${id}`),
    create: (body: DiscountInput) => api.post<ApiResponse<Discount>>('/discounts', body),
    update: (id: string, body: Partial<DiscountInput>) =>
      api.patch<ApiResponse<Discount>>(`/discounts/${id}`, body),
    activate: (id: string) => api.post<ApiResponse<Discount>>(`/discounts/${id}/activate`),
    deactivate: (id: string) => api.post<ApiResponse<Discount>>(`/discounts/${id}/deactivate`),
  },
}
