import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export const SERVICE_TYPES = ['DINE_IN', 'TAKEAWAY', 'PICKUP', 'DELIVERY'] as const
export type ServiceType = (typeof SERVICE_TYPES)[number]

export const SERVICE_TYPE_LABELS: Record<ServiceType, string> = {
  DINE_IN: 'Dine-in',
  TAKEAWAY: 'Takeaway',
  PICKUP: 'Pickup',
  DELIVERY: 'Delivery',
}

export const ORDER_STATUSES = [
  'DRAFT',
  'CONFIRMED',
  'IN_PROGRESS',
  'READY',
  'SERVED',
  'COMPLETED',
  'CANCELLED',
] as const

export interface OrderItemModifier {
  id: string
  option_id?: string
  group_name?: string
  option_name: string
  price_delta: string
  quantity: number
}

export interface OrderItem {
  id: string
  round_no: number
  product_id: string
  variant_id: string
  product_name: string
  variant_name: string
  sku?: string
  quantity: number
  unit_price: string
  modifiers_total: string
  discount_amount: string
  line_total: string
  unit_cogs: string
  line_cogs: string
  cost_unknown: boolean
  status: string
  note?: string
  modifiers?: OrderItemModifier[]
}

export interface Order {
  id: string
  order_no?: string
  queue_no?: number
  service_type: ServiceType
  status: string
  payment_status: string
  payment_timing: string
  table_id?: string
  customer_name?: string
  customer_phone?: string
  delivery_address?: string
  note?: string
  priority: string
  subtotal_amount: string
  item_discount_amount: string
  order_discount_amount: string
  service_charge_rate: string
  service_charge_amount: string
  tax_rate: string
  tax_amount: string
  tax_inclusive: boolean
  rounding_amount: string
  total_amount: string
  paid_amount: string
  total_cogs: string
  created_at: string
  confirmed_at?: string
  items?: OrderItem[]
}

export interface OrderPayment {
  id: string
  method_code: string
  amount: string
  status: string
  reference_no?: string
  paid_at?: string
}

export interface Receipt {
  outlet: {
    id: string
    code: string
    name: string
    address?: string
    phone?: string
  }
  order: Order
  payments: OrderPayment[] | null
}

export interface DraftInput {
  service_type: ServiceType
  table_id: string | null
  customer_name: string | null
  customer_phone: string | null
  delivery_address: string | null
  note: string | null
  priority: 'NORMAL' | 'RUSH'
}

export interface AddItemInput {
  product_id: string
  variant_id: string
  quantity: number
  modifier_option_ids: string[]
  note: string | null
}

export interface UpdateItemInput {
  quantity?: number
  note?: string | null
}

export interface CancelInput {
  reason_code: string
  note: string | null
  mode?: 'SALE_REVERSAL' | 'WASTE'
}

// Metode bayar seed di tabel payment_methods; belum ada endpoint daftar metode.
export const PAYMENT_METHODS = [
  { code: 'CASH', label: 'Tunai', isCash: true, requiresReference: false },
  { code: 'QRIS', label: 'QRIS', isCash: false, requiresReference: true },
  { code: 'TRANSFER', label: 'Transfer bank', isCash: false, requiresReference: true },
  { code: 'E_WALLET', label: 'E-wallet', isCash: false, requiresReference: true },
] as const

export const paymentMethodLabel = (code: string) =>
  PAYMENT_METHODS.find((method) => method.code === code)?.label ?? code

export interface PaymentInput {
  method_code: string
  amount: string
  cash_received: string | null
  reference_no: string | null
}

export interface PaymentResult {
  payment: OrderPayment & {
    order_id: string
    shift_id: string
    cash_received?: string
    change_amount?: string
    received_by: string
    created_at: string
  }
  order: {
    id: string
    order_no: string
    status: string
    payment_status: string
    total_amount: string
    paid_amount: string
    refunded_amount: string
    remaining: string
  }
}

export const orderApi = {
  list: (params: { q?: string; status?: string; page?: number; per_page?: number }) =>
    api.get<PaginatedResponse<Order>>('/orders', { params }),
  get: (id: string) => api.get<ApiResponse<Order>>(`/orders/${id}`),
  createDraft: (body: DraftInput) => api.post<ApiResponse<Order>>('/orders', body),
  addItem: (id: string, body: AddItemInput) =>
    api.post<ApiResponse<Order>>(`/orders/${id}/items`, body),
  updateItem: (id: string, itemID: string, body: UpdateItemInput) =>
    api.patch<ApiResponse<Order>>(`/orders/${id}/items/${itemID}`, body),
  deleteItem: (id: string, itemID: string) =>
    api.delete<ApiResponse<Order>>(`/orders/${id}/items/${itemID}`),
  confirm: (id: string, key: string) =>
    api.post<ApiResponse<Order>>(`/orders/${id}/confirm`, null, {
      headers: { 'Idempotency-Key': key },
    }),
  cancel: (id: string, body: CancelInput) =>
    api.post<ApiResponse<Order>>(`/orders/${id}/cancel`, body),
  receipt: (id: string) => api.get<ApiResponse<Receipt>>(`/orders/${id}/receipt`),
  reprintReceipt: (id: string) => api.post<ApiResponse<Receipt>>(`/orders/${id}/receipt/reprint`),
  payments: (id: string) => api.get<ApiResponse<OrderPayment[]>>(`/orders/${id}/payments`),
  createPayment: (id: string, key: string, body: PaymentInput) =>
    api.post<ApiResponse<PaymentResult>>(`/orders/${id}/payments`, body, {
      headers: { 'Idempotency-Key': key },
    }),
}
