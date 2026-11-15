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
    void refreshSession().then((token) => {
      // Gangguan jaringan tidak mengubah status di doRefresh(); lepaskan spinner
      // agar pengguna melihat halaman login, bukan loading tanpa akhir.
      if (!token) {
        const store = useAuthStore.getState()
        if (store.status === 'loading') store.setStatus('anonymous')
      }
    })
  }, [status, setStatus])

  return status
}
