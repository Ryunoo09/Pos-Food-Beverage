import { api } from './client'
import type { ApiResponse } from '../types/api.types'

export interface SupplierIngredient {
  ingredient_id: string
  ingredient_name?: string
  purchase_unit_id?: number
  last_price?: string
  lead_time_days?: number
  is_preferred: boolean
}

export interface Supplier {
  id: string
  name: string
  contact_person?: string
  phone?: string
  email?: string
  address?: string
  note?: string
  is_active: boolean
  ingredients: SupplierIngredient[]
}

export interface SupplierInput {
  name: string
  contact_person: string | null
  phone: string | null
  email: string | null
  address: string | null
  note: string | null
}

export interface SupplierIngredientInput {
  ingredient_id: string
  purchase_unit_id: number | null
  last_price: string | null
  lead_time_days: number | null
  is_preferred: boolean
}

export interface PurchaseHistoryRow {
  id: string
  code: string
  status: string
  order_date: string
  total_amount: string
  item_count: number
}

export const supplierApi = {
  list: (params: { q?: string; is_active?: boolean } = {}) =>
    api.get<ApiResponse<Supplier[]>>('/suppliers', { params }),
  get: (id: string) => api.get<ApiResponse<Supplier>>(`/suppliers/${id}`),
  create: (body: SupplierInput) => api.post<ApiResponse<Supplier>>('/suppliers', body),
  update: (id: string, body: SupplierInput) =>
    api.patch<ApiResponse<Supplier>>(`/suppliers/${id}`, body),
  activate: (id: string) => api.post<void>(`/suppliers/${id}/activate`),
  deactivate: (id: string) => api.post<void>(`/suppliers/${id}/deactivate`),
  setIngredients: (id: string, ingredients: SupplierIngredientInput[]) =>
    api.put<void>(`/suppliers/${id}/ingredients`, { ingredients }),
  purchaseHistory: (id: string) =>
    api.get<ApiResponse<PurchaseHistoryRow[]>>(`/suppliers/${id}/purchase-history`),
}
