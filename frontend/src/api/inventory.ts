import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface Unit {
  id: number
  code: string
  name: string
  dimension: string
  to_base_factor: string
  is_base: boolean
}

export interface IngredientCategory {
  id: string
  name: string
  is_active: boolean
}

export interface Ingredient {
  id: string
  code: string
  name: string
  category_id?: string
  category_name?: string
  base_unit_id: number
  base_unit_code: string
  default_purchase_unit_id?: number
  min_stock: string
  is_active: boolean
  is_stock_product: boolean
  quantity_on_hand: string
  avg_unit_cost: string
}

export interface IngredientInput {
  code: string
  name: string
  category_id: string | null
  base_unit_id: number
  default_purchase_unit_id: number | null
  min_stock: string
  is_stock_product: boolean
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

export const inventoryApi = {
  units: () => api.get<ApiResponse<Unit[]>>('/units'),
  categories: {
    list: () => api.get<ApiResponse<IngredientCategory[]>>('/ingredient-categories'),
    create: (body: { name: string }) =>
      api.post<ApiResponse<IngredientCategory>>('/ingredient-categories', body),
  },
  ingredients: {
    list: (params: { q?: string; is_active?: boolean; page?: number; per_page?: number }) =>
      api.get<PaginatedResponse<Ingredient>>('/ingredients', { params }),
    get: (id: string) => api.get<ApiResponse<Ingredient>>(`/ingredients/${id}`),
    create: (body: IngredientInput) => api.post<ApiResponse<Ingredient>>('/ingredients', body),
    update: (id: string, body: IngredientInput) =>
      api.patch<ApiResponse<Ingredient>>(`/ingredients/${id}`, body),
    activate: (id: string) => api.post<void>(`/ingredients/${id}/activate`),
    deactivate: (id: string) => api.post<void>(`/ingredients/${id}/deactivate`),
    movements: (id: string) => api.get<ApiResponse<StockMovement[]>>(`/ingredients/${id}/movements`),
  },
  stock: {
    list: (params: { q?: string; low?: boolean; page?: number; per_page?: number }) =>
      api.get<PaginatedResponse<StockLevel>>('/inventory', { params }),
  },
}
