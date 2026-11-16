import { api } from './client'
import type { ApiResponse } from '../types/api.types'

/*
 * Laporan adalah estimasi operasional, bukan laporan akuntansi resmi.
 * Semua nominal berupa string dari server; klien tidak menghitung uang.
 */

export interface SalesTotals {
  order_count: number
  gross_sales: string
  discount: string
  service_charge: string
  tax: string
  net_sales: string
  total: string
  aov: string
}

export interface SalesBucket {
  key: string
  label: string
  order_count: number
  gross_sales: string
  net_sales: string
  total: string
}

export interface ProductLine {
  product_id: string
  name: string
  quantity: number
  revenue: string
  gross_profit: string
  margin_percent: string
}

export interface SalesReport {
  from: string
  to: string
  total: SalesTotals
  daily: SalesBucket[]
  by_service_type: SalesBucket[]
  by_payment_method: SalesBucket[]
  by_cashier: SalesBucket[]
  top_products: ProductLine[]
}

export interface ExpenseLine {
  category_name: string
  amount: string
}

export interface FinancialReport {
  from: string
  to: string
  gross_sales: string
  discount: string
  service_charge: string
  tax: string
  net_sales: string
  /** Hanya ada bila pengguna punya reports.profit.view. */
  hpp?: string
  gross_profit?: string
  waste?: string
  operating_expense?: string
  net_profit?: string
  gross_margin_percent?: string
  expenses_by_category?: ExpenseLine[]
}

export interface StockLine {
  ingredient_id: string
  code: string
  name: string
  unit: string
  quantity_on_hand: string
  min_stock: string
  avg_unit_cost: string
  value: string
  status: string
}

export interface MovementLine {
  ingredient_id: string
  name: string
  movement_type: string
  quantity: string
}

export interface WasteLine {
  reason_code: string
  count: number
  cost_value: string
}

export interface OpnameLine {
  id: string
  opname_no: string
  status: string
  finished_at: string
  total_difference_quantity: string
  total_difference_value: string
}

export interface InventoryReport {
  from: string
  to: string
  stock_value: string
  waste_value: string
  stock: StockLine[]
  movements: MovementLine[]
  waste: WasteLine[]
  opnames: OpnameLine[]
}

export interface MenuPerformance {
  product_id: string
  name: string
  variant_name: string
  quantity: number
  revenue: string
  total_cost: string
  gross_profit: string
  margin_percent: string | null
}

export interface MenuPerformanceReport {
  from: string
  to: string
  items: MenuPerformance[]
}

export interface ShiftLine {
  id: string
  cashier_name: string
  status: string
  opened_at: string
  closed_at?: string
  opening_cash: string
  expected_cash?: string
  actual_cash?: string
  difference?: string
  sales: string
  payment_methods: { method: string; amount: string }[]
}

export interface ShiftReport {
  from: string
  to: string
  shifts: ShiftLine[]
}

export type ReportKind = 'sales' | 'financial' | 'inventory' | 'menu-performance' | 'shifts'

interface Period {
  from: string
  to: string
}

export const reportApi = {
  sales: (params: Period) => api.get<ApiResponse<SalesReport>>('/reports/sales', { params }),
  financial: (params: Period) =>
    api.get<ApiResponse<FinancialReport>>('/reports/financial', { params }),
  inventory: (params: Period) =>
    api.get<ApiResponse<InventoryReport>>('/reports/inventory', { params }),
  menuPerformance: (params: Period) =>
    api.get<ApiResponse<MenuPerformanceReport>>('/reports/menu-performance', { params }),
  shifts: (params: Period) => api.get<ApiResponse<ShiftReport>>('/reports/shifts', { params }),
  // CSV diunduh lewat blob agar Authorization header tetap terkirim (bukan <a href>).
  exportCsv: (kind: ReportKind, params: Period) =>
    api.get<Blob>(`/reports/${kind}/export`, { params, responseType: 'blob' }),
}

export const DISCLAIMER =
  'Laporan ini adalah estimasi operasional, bukan laporan akuntansi resmi.'
