import axios from 'axios'
import { useAuthStore } from '../store/auth'
import type { ApiError, ApiResponse } from '../types/api.types'
import type { AuthPayload } from '../types/auth.types'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? '/api/v1',
  withCredentials: true, // refresh token dikirim sebagai cookie HttpOnly
})

// Endpoint yang 401-nya bukan tanda access token kedaluwarsa.
const AUTH_EXEMPT = ['/auth/login', '/auth/refresh']
const RETRY_HEADER = 'X-Auth-Retry'

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let refreshing: Promise<string | null> | null = null

/** Refresh sekali untuk semua request yang kena 401 bersamaan. */
export function refreshSession(): Promise<string | null> {
  refreshing ??= axios
    .post<ApiResponse<AuthPayload>>(`${api.defaults.baseURL}/auth/refresh`, null, {
      withCredentials: true,
    })
    .then(({ data }) => {
      useAuthStore.getState().setAuth(data.data.user, data.data.access_token, data.data.permissions)
      return data.data.access_token
    })
    .catch(() => {
      useAuthStore.getState().clearAuth()
      return null
    })
    .finally(() => {
      refreshing = null
    })
  return refreshing
}

api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    const config = axios.isAxiosError(error) ? error.config : undefined
    const isExpired =
      axios.isAxiosError(error) &&
      error.response?.status === 401 &&
      config !== undefined &&
      config.headers?.get?.(RETRY_HEADER) === undefined &&
      !AUTH_EXEMPT.some((path) => config.url?.includes(path))

    if (isExpired) {
      const token = await refreshSession()
      if (token) {
        config.headers.set('Authorization', `Bearer ${token}`)
        config.headers.set(RETRY_HEADER, '1')
        return api.request(config)
      }
      redirectToLogin()
    }

    throw toApiError(error)
  },
)

function redirectToLogin() {
  if (window.location.pathname !== '/login') window.location.assign('/login')
}

function toApiError(error: unknown): ApiError {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data as { error?: Partial<ApiError> } | undefined
    const parsed = body?.error
    if (parsed?.code && parsed.message) {
      return {
        code: parsed.code,
        message: parsed.message,
        details: parsed.details,
        retryAfter: retryAfter(error.response?.headers['retry-after']),
      }
    }
    if (error.response?.status === 401) {
      return { code: 'UNAUTHENTICATED', message: 'Sesi berakhir. Silakan masuk kembali.' }
    }
    return {
      code: 'NETWORK_ERROR',
      message: 'Tidak dapat terhubung ke server.',
      details: error.message,
    }
  }
  return { code: 'UNKNOWN_ERROR', message: 'Terjadi kesalahan tidak terduga.' }
}

function retryAfter(header: unknown): number | undefined {
  const seconds = Number(header)
  return Number.isFinite(seconds) && seconds > 0 ? seconds : undefined
}
