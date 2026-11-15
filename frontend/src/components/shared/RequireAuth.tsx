import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useSession } from '../../hooks/useSession'
import { useAuthStore } from '../../store/auth'
import { Spinner } from '../ui/Spinner'

export function RequireAuth() {
  const status = useSession()
  const mustChangePassword = useAuthStore((state) => state.user?.must_change_password)
  const location = useLocation()

  if (status === 'idle' || status === 'loading') {
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner label="Memuat sesi" />
      </div>
    )
  }

  if (status === 'anonymous') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  if (mustChangePassword && location.pathname !== '/change-password') {
    return <Navigate to="/change-password" replace />
  }

  return <Outlet />
}
