import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export const PO_STATUSES = [
  'DRAFT',
  'SUBMITTED',
  'APPROVED',
  'PARTIALLY_RECEIVED',
  'RECEIVED',
  'CANCELLED',
] as const

export const PO_STATUS_LABELS: Record<string, string> = {
  DRAFT: 'Draft',
  SUBMITTED: 'Diajukan',
  APPROVED: 'Disetujui',
  PARTIALLY_RECEIVED: 'Diterima sebagian',
  RECEIVED: 'Diterima',
  CANCELLED: 'Dibatalkan',
}

export interface PurchaseOrderItem {
  id: string
  ingredient_id: string
  ingredient_code: string
  ingredient_name: string
  purchase_unit_id: number
  purchase_unit_code: string
  quantity_ordered: string
  unit_price: string
  discount_amount: string
  line_total: string
  base_quantity_ordered: string
  quantity_received_base: string
}

export interface PurchaseOrder {
  id: string
  code: string
  supplier_id: string
  supplier_name: string
  status: string
  order_date: string
  expected_date?: string
  subtotal: string
  discount_amount: string
  total_amount: string
  note?: string
  version: number
  created_at: string
  /** Pengaju PO; dipakai UI untuk menyembunyikan Setujui dari pengaju sendiri. */
  submitted_by?: string
  items: PurchaseOrderItem[] | null
}

export interface PurchaseOrderItemInput {
  ingredient_id: string
  purchase_unit_id: number
  quantity_ordered: string
  unit_price: string
  discount_amount: string
}

export interface PurchaseOrderInput {
  supplier_id: string
  expected_date: string | null
  note: string | null
  discount_amount: string
  items: PurchaseOrderItemInput[]
}

export interface PurchaseOrderUpdate {
  expected_date?: string | null
  note?: string | null
  discount_amount?: string
  items?: PurchaseOrderItemInput[]
}

export interface ReceivingItem {
  id: string
  ingredient_id: string
  ingredient_code: string
  ingredient_name: string
  quantity_received: string
  base_quantity: string
  unit_price: string
  net_unit_cost_base: string
  line_total: string
}

export interface Receiving {
  id: string
  code: string
  purchase_order_id?: string
  purchase_order_code?: string
  supplier_id: string
  supplier_name: string
  supplier_invoice_no?: string
  note?: string
  received_at: string
  items: ReceivingItem[] | null
}

export interface ReceivingInput {
  supplier_id: string
  supplier_invoice_no: string | null
  note: string | null
  items: {
    po_item_id: string | null
    ingredient_id: string
    purchase_unit_id: number
    quantity_received: string
    unit_price: string
    discount_allocated: string
  }[]
}

export const purchaseApi = {
  list: (params: { status?: string; supplier_id?: string; page?: number; per_page?: number } = {}) =>
    api.get<PaginatedResponse<PurchaseOrder>>('/purchase-orders', { params }),
  get: (id: string) => api.get<ApiResponse<PurchaseOrder>>(`/purchase-orders/${id}`),
  create: (body: PurchaseOrderInput) =>
    api.post<ApiResponse<PurchaseOrder>>('/purchase-orders', body),
  update: (id: string, body: PurchaseOrderUpdate) =>
    api.patch<ApiResponse<PurchaseOrder>>(`/purchase-orders/${id}`, body),
  submit: (id: string) => api.post<ApiResponse<PurchaseOrder>>(`/purchase-orders/${id}/submit`),
  approve: (id: string) => api.post<ApiResponse<PurchaseOrder>>(`/purchase-orders/${id}/approve`),
  cancel: (id: string, reason: string) =>
    api.post<ApiResponse<PurchaseOrder>>(`/purchase-orders/${id}/cancel`, { reason }),
  receive: (id: string, key: string, body: ReceivingInput) =>
    api.post<ApiResponse<Receiving>>(`/purchase-orders/${id}/receivings`, body, {
      headers: { 'Idempotency-Key': key },
    }),
  receiveDirect: (key: string, body: ReceivingInput) =>
    api.post<ApiResponse<Receiving>>('/purchase-receivings', body, {
      headers: { 'Idempotency-Key': key },
    }),
  receiving: (id: string) => api.get<ApiResponse<Receiving>>(`/purchase-receivings/${id}`),
}
