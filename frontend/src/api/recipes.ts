import { api } from './client'
import type { ApiResponse } from '../types/api.types'

export interface RecipeItem {
  ingredient_id: string
  ingredient_name: string
  unit_code: string
  quantity: string
  waste_factor_percent: string
  line_cost?: string
}

export interface Recipe {
  id: string
  variant_id: string
  version_no: number
  status: string
  yield_note?: string
  items: RecipeItem[]
  total_cost?: string
}

export interface RecipeItemInput {
  ingredient_id: string
  quantity: string
  waste_factor_percent: string
}

export interface RecipeInput {
  yield_note: string | null
  items: RecipeItemInput[]
}

export const RECIPE_STATUS_LABELS: Record<string, string> = {
  ACTIVE: 'Aktif',
  SUPERSEDED: 'Digantikan',
}

export const recipeApi = {
  // Biaya baris hanya dikirim backend bila user punya izin recipes.view_cost.
  active: (variantID: string) =>
    api.get<ApiResponse<Recipe>>(`/variants/${variantID}/recipe`),
  versions: (variantID: string) =>
    api.get<ApiResponse<Recipe[]>>(`/variants/${variantID}/recipes`),
  create: (variantID: string, body: RecipeInput) =>
    api.post<ApiResponse<Recipe>>(`/variants/${variantID}/recipes`, body),
}
