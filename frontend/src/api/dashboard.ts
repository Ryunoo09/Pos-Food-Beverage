import { api } from './client'
import type { ApiResponse } from '../types/api.types'

/*
 * Ringkasan dashboard. Semua nominal berupa string dari server — klien tidak
 * melakukan aritmetika uang. HPP dan laba kotor hanya dikirim backend bila
 * pengguna punya reports.profit.view, karena itu opsional.
 */
export interface DashboardSummary {
  from: string
  to: string
  gross_sales: string
  net_sales: string
  transactions: number
  aov: string
  hpp?: string
  gross_profit?: string
  payment_methods: { method: string; amount: string }[]
  hourly_sales: { hour: number; sales: string; transactions: number }[]
  top_products: { product_id: string; name: string; quantity: number; sales: string }[]
  critical_stock: {
    ingredient_id: string
    name: string
    quantity_on_hand: string
    min_stock: string
  }[]
  active_orders: { status: string; count: number }[]
}

export const dashboardApi = {
  summary: (params: { from: string; to: string }) =>
    api.get<ApiResponse<DashboardSummary>>('/dashboard', { params }),
}

/** Label metode bayar yang dikenal; kode tak dikenal ditampilkan apa adanya. */
export const PAYMENT_METHOD_LABELS: Record<string, string> = {
  CASH: 'Tunai',
  QRIS: 'QRIS',
  TRANSFER: 'Transfer bank',
  E_WALLET: 'E-wallet',
}

/** Pilihan untuk form; dibatasi pada kode aktif yang ada di payment_methods. */
export const PAYMENT_METHOD_OPTIONS = [
  { value: 'CASH', label: 'Tunai' },
  { value: 'QRIS', label: 'QRIS' },
  { value: 'TRANSFER', label: 'Transfer bank' },
  { value: 'E_WALLET', label: 'E-wallet' },
] as const
