import axios from 'axios'
import { useAuthStore } from '../store/auth'
import type { ApiError } from '../types/api.types'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? '/api/v1',
  withCredentials: true,
})

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    throw toApiError(error)
  },
)

function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data as { error?: Partial<ApiError> } | undefined
    const parsed = body?.error
    if (parsed?.code && parsed.message) {
      return { code: parsed.code, message: parsed.message, details: parsed.details }
    }
    return {
      code: 'NETWORK_ERROR',
      message: 'Tidak dapat terhubung ke server.',
      details: error.message,
    }
  }
  return { code: 'UNKNOWN_ERROR', message: 'Terjadi kesalahan tidak terduga.' }
}
