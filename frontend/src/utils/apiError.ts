import type { ApiError } from '../types/api.types'

/** Pesan siap tampil dari error API; tidak pernah menampilkan "error" generik. */
export function apiMessage(error: unknown): string {
  const apiError = error as Partial<ApiError> | null
  return apiError?.message ?? 'Terjadi kesalahan. Coba lagi.'
}
