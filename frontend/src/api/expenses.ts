import { api } from './client'
import type { ApiResponse, PaginatedResponse } from '../types/api.types'

export interface ExpenseCategory {
  id: string
  name: string
  is_active: boolean
  created_at: string
}

export interface Expense {
  id: string
  category_id: string
  category_name: string
  name: string
  amount: string
  expense_date: string
  payment_method_code?: string
  paid_from_drawer: boolean
  shift_id?: string
  note?: string
  is_voided: boolean
  void_reason?: string
  recorded_by?: string
  created_at: string
}

export interface ExpenseInput {
  category_id: string
  name: string
  amount: string
  expense_date: string
  payment_method_code: string | null
  paid_from_drawer: boolean
  note: string | null
}

export const expenseApi = {
  list: (params: {
    from?: string
    to?: string
    category_id?: string
    include_voided?: boolean
    page?: number
    per_page?: number
  }) => api.get<PaginatedResponse<Expense>>('/expenses', { params }),
  create: (body: ExpenseInput) => api.post<ApiResponse<Expense>>('/expenses', body),
  update: (id: string, body: Partial<ExpenseInput>) =>
    api.patch<ApiResponse<Expense>>(`/expenses/${id}`, body),
  void: (id: string, reason: string) =>
    api.post<ApiResponse<Expense>>(`/expenses/${id}/void`, { reason }),
  categories: {
    list: () => api.get<ApiResponse<ExpenseCategory[]>>('/expense-categories'),
    create: (body: { name: string; is_active?: boolean }) =>
      api.post<ApiResponse<ExpenseCategory>>('/expense-categories', body),
    update: (id: string, body: { name?: string; is_active?: boolean }) =>
      api.patch<ApiResponse<ExpenseCategory>>(`/expense-categories/${id}`, body),
  },
}
