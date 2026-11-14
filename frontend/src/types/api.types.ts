export interface ApiResponse<T> {
  data: T
  meta: { request_id: string }
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: {
    page: number
    per_page: number
    total: number
    total_pages: number
    request_id: string
  }
}

export interface ApiError {
  code: string
  message: string
  details?: unknown
}
