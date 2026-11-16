import { api } from './client'
import type { ApiResponse } from '../types/api.types'

export const KITCHEN_STATUSES = ['NEW', 'COOKING', 'READY', 'SERVED'] as const
export type KitchenStatus = (typeof KITCHEN_STATUSES)[number]

export const KITCHEN_STATUS_LABELS: Record<KitchenStatus, string> = {
  NEW: 'Baru',
  COOKING: 'Dimasak',
  READY: 'Siap',
  SERVED: 'Disajikan',
}

export interface KitchenItem {
  id: string
  order_item_id?: string
  name: string
  variant?: string
  modifiers_text?: string
  note?: string
  quantity: number
  station?: string
  status: KitchenStatus
  started_at?: string
  ready_at?: string
  served_at?: string
  version: number
}

export interface KitchenTicket {
  id: string
  order_id: string
  order_no?: string
  queue_no?: number
  service_type: string
  table_name?: string
  round_no: number
  status: KitchenStatus
  priority: string
  received_at: string
  started_at?: string
  ready_at?: string
  served_at?: string
  version: number
  items: KitchenItem[]
}

export const kitchenApi = {
  list: (params: { status?: string; station?: string } = {}) =>
    api.get<ApiResponse<KitchenTicket[]>>('/kitchen/orders', { params }),
  updateItemStatus: (itemID: string, status: KitchenStatus, version: number) =>
    api.patch<ApiResponse<KitchenTicket>>(`/kitchen/items/${itemID}/status`, { status, version }),
  setPriority: (orderID: string, priority: 'NORMAL' | 'RUSH') =>
    api.patch<ApiResponse<KitchenTicket>>(`/kitchen/orders/${orderID}/priority`, { priority }),
}
