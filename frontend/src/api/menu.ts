import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface Category {
  id: string
  name: string
  sort_order: number
  is_active: boolean
  created_at: string
}

export interface CategoryInput {
  name: string
  sort_order: number
  is_active?: boolean
}

export interface Variant {
  id: string
  name: string
  sku?: string
  price: string
  is_default: boolean
  is_active: boolean
  sort_order: number
}

export interface Product {
  id: string
  category_id: string
  category_name?: string
  sku: string
  name: string
  description?: string
  product_type: string
  stock_mode: string
  stock_ingredient_id?: string
  kitchen_station?: string
  allow_zero_price: boolean
  is_active: boolean
  is_available: boolean
  sort_order: number
  price: string
  variants?: Variant[]
  modifier_group_ids?: string[]
}

export type ProductInput = Omit<
  Product,
  | 'id'
  | 'category_name'
  | 'price'
  | 'variants'
  | 'modifier_group_ids'
  | 'description'
  | 'stock_ingredient_id'
  | 'kitchen_station'
  | 'is_active'
  | 'is_available'
> & {
  description?: string | null
  stock_ingredient_id?: string | null
  kitchen_station?: string | null
  variants?: VariantInput[]
}

export interface VariantInput {
  name: string
  sku: string
  price: string
  is_default: boolean
  is_active?: boolean
  sort_order: number
}

export interface ModifierOption {
  id: string
  name: string
  price_delta: string
  is_default: boolean
  is_active: boolean
  sort_order: number
}

export interface ModifierOptionInput {
  id?: string
  name: string
  price_delta: string
  is_default: boolean
  sort_order: number
}

export interface ModifierGroup {
  id: string
  name: string
  min_select: number
  max_select: number
  is_required: boolean
  sort_order: number
  is_active: boolean
  options: ModifierOption[]
}

export interface ModifierGroupInput {
  name: string
  min_select: number
  max_select: number
  is_required: boolean
  sort_order: number
  is_active?: boolean
  options: ModifierOptionInput[]
}

export interface PriceHistory {
  old_price: string | null
  new_price: string
  reason?: string
  changed_at: string
}

export interface ProductFilter {
  q?: string
  category_id?: string
  is_active?: boolean
  available?: boolean
  page?: number
  per_page?: number
}

export const menuApi = {
  categories: {
    list: () => api.get<ApiResponse<Category[]>>('/categories'),
    create: (body: CategoryInput) => api.post<ApiResponse<Category>>('/categories', body),
    update: (id: string, body: CategoryInput) =>
      api.patch<ApiResponse<Category>>(`/categories/${id}`, body),
    remove: (id: string) => api.delete<void>(`/categories/${id}`),
  },
  products: {
    list: (params: ProductFilter) => api.get<PaginatedResponse<Product>>('/products', { params }),
    get: (id: string) => api.get<ApiResponse<Product>>(`/products/${id}`),
    create: (body: ProductInput) => api.post<ApiResponse<Product>>('/products', body),
    update: (id: string, body: ProductInput) =>
      api.patch<ApiResponse<Product>>(`/products/${id}`, body),
    setAvailability: (id: string, is_available: boolean) =>
      api.patch<void>(`/products/${id}/availability`, { is_available }),
    activate: (id: string) => api.post<void>(`/products/${id}/activate`),
    deactivate: (id: string) => api.post<void>(`/products/${id}/deactivate`),
  },
  variants: {
    list: (productId: string) => api.get<ApiResponse<Variant[]>>(`/products/${productId}/variants`),
    create: (productId: string, body: VariantInput) =>
      api.post<ApiResponse<Variant>>(`/products/${productId}/variants`, body),
    update: (id: string, body: VariantInput) =>
      api.patch<ApiResponse<Variant>>(`/variants/${id}`, body),
    priceHistory: (id: string) => api.get<ApiResponse<PriceHistory[]>>(`/variants/${id}/price-history`),
  },
  modifiers: {
    list: () => api.get<ApiResponse<ModifierGroup[]>>('/modifier-groups'),
    create: (body: ModifierGroupInput) =>
      api.post<ApiResponse<ModifierGroup>>('/modifier-groups', body),
    update: (id: string, body: ModifierGroupInput) =>
      api.patch<ApiResponse<ModifierGroup>>(`/modifier-groups/${id}`, body),
    activate: (id: string) => api.post<void>(`/modifier-groups/${id}/activate`),
    deactivate: (id: string) => api.post<void>(`/modifier-groups/${id}/deactivate`),
    setProductGroups: (productId: string, group_ids: string[]) =>
      api.put<void>(`/products/${productId}/modifier-groups`, { group_ids }),
  },
}
