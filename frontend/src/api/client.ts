import axios from 'axios'
import { useAuthStore } from '../store/auth'
import type { ApiError, ApiResponse } from '../types/api.types'
import type { AuthPayload } from '../types/auth.types'

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? '/api/v1',
  withCredentials: true, // refresh token dikirim sebagai cookie HttpOnly
  timeout: 15_000,
})

// Endpoint yang 401-nya bukan tanda access token kedaluwarsa.
const AUTH_EXEMPT = ['/auth/login', '/auth/refresh']
const RETRY_HEADER = 'X-Auth-Retry'
const REFRESH_LOCK = 'pos-auth-refresh'

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().accessToken
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let refreshing: Promise<string | null> | null = null

/**
 * Refresh sekali untuk semua request yang kena 401 bersamaan.
 *
 * Backend merotasi refresh token dan mencabut seluruh family bila token lama
 * dipakai ulang. Karena itu refresh diserialkan antar-tab lewat Web Locks:
 * tanpa lock, dua tab yang boot bersamaan memakai cookie yang sama dan tab
 * kedua memicu AUTH_REFRESH_REUSED (semua sesi ikut dicabut).
 */
export function refreshSession(): Promise<string | null> {
  refreshing ??= withRefreshLock(doRefresh).finally(() => {
    refreshing = null
  })
  return refreshing
}

async function doRefresh(): Promise<string | null> {
  try {
    const { data } = await axios.post<ApiResponse<AuthPayload>>(
      `${api.defaults.baseURL}/auth/refresh`,
      null,
      { withCredentials: true, timeout: 10_000 },
    )
    useAuthStore.getState().setAuth(data.data.user, data.data.access_token, data.data.permissions)
    return data.data.access_token
  } catch (error) {
    // Gangguan jaringan tidak boleh menghapus sesi yang masih sah; hanya
    // penolakan eksplisit dari server yang membersihkan state.
    const status = axios.isAxiosError(error) ? error.response?.status : undefined
    if (status === 401 || status === 403) useAuthStore.getState().clearAuth()
    return null
  }
}

function withRefreshLock<T>(task: () => Promise<T>): Promise<T> {
  const locks = navigator.locks
  return locks ? locks.request(REFRESH_LOCK, task) : task()
}

api.interceptors.response.use(
  (response) => response,
  async (error: unknown) => {
    const config = axios.isAxiosError(error) ? error.config : undefined
    const path = config?.url?.split('?')[0] ?? ''
    const isExpired =
      axios.isAxiosError(error) &&
      error.response?.status === 401 &&
      config !== undefined &&
      config.headers?.get?.(RETRY_HEADER) === undefined &&
      !AUTH_EXEMPT.includes(path)

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
    if (error.code === 'ECONNABORTED') {
      return { code: 'TIMEOUT', message: 'Server tidak merespons. Coba lagi.' }
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
