import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface StockLevel {
  ingredient_id: string
  code: string
  name: string
  base_unit_code: string
  quantity_on_hand: string
  min_stock: string
  is_low: boolean
  avg_unit_cost: string
  stock_value: string
}

export interface StockMovement {
  id: string
  movement_type: string
  quantity_delta: string
  unit_cost: string
  total_cost: string
  balance_after: string
  avg_cost_after: string
  reference_type: string
  reference_id?: string
  is_shortage: boolean
  reason?: string
  created_at: string
}

export interface AdjustmentInput {
  quantity_delta: string
  reason: string
  note: string | null
}

// Alasan penyesuaian manual; backend menyimpan teks bebas, UI memakai pilihan tetap
// agar laporan mudah dibaca.
export const ADJUSTMENT_REASONS = [
  { value: 'STOCK_COUNT', label: 'Hasil hitung fisik' },
  { value: 'DAMAGED', label: 'Rusak / tumpah' },
  { value: 'EXPIRED', label: 'Kedaluwarsa' },
  { value: 'DATA_CORRECTION', label: 'Koreksi data' },
] as const

export const MOVEMENT_LABELS: Record<string, string> = {
  INITIAL_STOCK: 'Stok awal',
  PURCHASE: 'Pembelian',
  SALE: 'Penjualan',
  SALE_REVERSAL: 'Pembatalan penjualan',
  WASTE: 'Waste',
  WASTE_REVERSAL: 'Pembatalan waste',
  ADJUSTMENT: 'Penyesuaian',
  RETURN: 'Retur',
  TRANSFER_OUT: 'Transfer keluar',
  TRANSFER_IN: 'Transfer masuk',
}

export const movementLabel = (type: string) => MOVEMENT_LABELS[type] ?? type

export const stockApi = {
  list: (params: { q?: string; low?: boolean; page?: number; per_page?: number }) =>
    api.get<PaginatedResponse<StockLevel>>('/inventory', { params }),
  movements: (ingredientID: string) =>
    api.get<ApiResponse<StockMovement[]>>(`/inventory/${ingredientID}/movements`),
  adjust: (ingredientID: string, body: AdjustmentInput) =>
    api.post<ApiResponse<StockLevel>>(`/inventory/${ingredientID}/adjust`, body),
}
