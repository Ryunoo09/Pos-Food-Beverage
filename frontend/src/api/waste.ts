import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export const WASTE_TYPES = ['INGREDIENT', 'PRODUCT'] as const
export type WasteType = (typeof WASTE_TYPES)[number]

export const WASTE_TYPE_LABELS: Record<string, string> = {
  INGREDIENT: 'Bahan',
  PRODUCT: 'Produk',
}

// Alasan baku backend belum divalidasi ketat; UI memakai daftar tetap agar
// laporan bisa dikelompokkan.
export const WASTE_REASONS = [
  { value: 'EXPIRED', label: 'Kedaluwarsa' },
  { value: 'DAMAGED', label: 'Rusak' },
  { value: 'SPILLED', label: 'Tumpah' },
  { value: 'MISTAKE', label: 'Salah buat' },
  { value: 'CUSTOMER_RETURN', label: 'Dikembalikan pelanggan' },
] as const

// Data lama sempat memakai kode berbahasa Indonesia untuk alasan yang sama.
// Tanpa alias ini laporan menampilkan kode mentah seperti "RUSAK".
const REASON_ALIASES: Record<string, string> = {
  RUSAK: 'DAMAGED',
  TUMPAH: 'SPILLED',
  KEDALUWARSA: 'EXPIRED',
  SALAH_BUAT: 'MISTAKE',
}

export const wasteReasonLabel = (code?: string) => {
  if (!code) return '-'
  const normalized = REASON_ALIASES[code] ?? code
  return WASTE_REASONS.find((reason) => reason.value === normalized)?.label ?? code
}

export interface WasteRecord {
  id: string
  waste_type: string
  ingredient_id?: string
  ingredient_code?: string
  ingredient_name?: string
  variant_id?: string
  variant_name?: string
  quantity: string
  unit_code?: string
  cost_value: string
  reason_code?: string
  note?: string
  order_id?: string
  status: string
  recorded_at: string
}

export interface WasteInput {
  waste_type: WasteType
  ingredient_id: string | null
  variant_id: string | null
  quantity: string
  unit_id: number | null
  reason_code: string | null
  note: string | null
  order_id: string | null
}

export const wasteApi = {
  list: (params: {
    waste_type?: string
    reason_code?: string
    from?: string
    to?: string
    page?: number
    per_page?: number
  }) => api.get<PaginatedResponse<WasteRecord>>('/waste-records', { params }),
  get: (id: string) => api.get<ApiResponse<WasteRecord>>(`/waste-records/${id}`),
  create: (body: WasteInput) => api.post<ApiResponse<WasteRecord>>('/waste-records', body),
  reverse: (id: string) => api.post<ApiResponse<WasteRecord>>(`/waste-records/${id}/reverse`),
}
