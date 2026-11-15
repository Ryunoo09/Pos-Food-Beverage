import { useEffect } from 'react'
import { refreshSession } from '../api/client'
import { useAuthStore } from '../store/auth'

/**
 * Memulihkan sesi saat aplikasi dibuka: access token hanya ada di memori, jadi
 * satu-satunya cara tahu sesi masih hidup adalah mencoba refresh lewat cookie.
 * refreshSession() sendiri yang mengisi atau membersihkan store.
 */
export function useSession() {
  const status = useAuthStore((state) => state.status)
  const setStatus = useAuthStore((state) => state.setStatus)

  useEffect(() => {
    if (status !== 'idle') return
    setStatus('loading')
    void refreshSession()
  }, [status, setStatus])

  return status
}
