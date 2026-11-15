import { LogOut, X } from 'lucide-react'
import { NavLink, useNavigate } from 'react-router-dom'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../store/auth'
import { cn } from '../../utils/cn'
import { navGroups } from './nav'
import { ProductMark } from './ProductMark'

export interface SidebarProps {
  open: boolean
  onClose: () => void
}

export function Sidebar({ open, onClose }: SidebarProps) {
  const permissions = useAuthStore((state) => state.permissions)
  const user = useAuthStore((state) => state.user)
  const clearAuth = useAuthStore((state) => state.clearAuth)
  const navigate = useNavigate()

  async function handleLogout() {
    await authApi.logout().catch(() => undefined)
    clearAuth()
    navigate('/login', { replace: true })
  }

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 z-40 bg-ink/40 lg:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-57 flex-col bg-ground transition-transform lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-14 items-center gap-2.5 px-4">
          <ProductMark className="size-5 shrink-0 text-brand-600" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm leading-tight font-semibold">Food POS</p>
            <p className="truncate text-[11px] leading-tight text-muted">Kasir &amp; dapur</p>
          </div>
          <button
            type="button"
            className="-mr-2 grid size-11 place-items-center rounded-control text-muted hover:bg-raised lg:hidden"
            onClick={onClose}
            aria-label="Tutup menu"
          >
            <X className="size-5" strokeWidth={1.5} />
          </button>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 pb-4">
          {navGroups.map((group) => {
            const items = group.items.filter((item) => permissions.includes(item.permission))
            if (items.length === 0) return null
            return (
              <div key={group.label}>
                <p className="px-2.5 pt-4 pb-1 text-xs text-muted">{group.label}</p>
                <ul>
                  {items.map(({ label, to, icon: Icon }) => (
                    <li key={to}>
                      <NavLink
                        to={to}
                        onClick={onClose}
                        className={({ isActive }) =>
                          cn(
                            'flex min-h-11 items-center gap-2.5 rounded-control px-2.5 text-sm',
                            isActive
                              ? 'bg-active font-medium text-ink'
                              : 'text-muted hover:bg-raised hover:text-ink',
                          )
                        }
                      >
                        <Icon className="size-4 shrink-0" strokeWidth={1.5} />
                        <span className="truncate">{label}</span>
                      </NavLink>
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </nav>

        <div className="border-t border-line px-2 py-2">
          <div className="flex items-center gap-2.5 rounded-control px-2 py-1.5">
            <span className="grid size-7 shrink-0 place-items-center rounded-full bg-raised text-[11px] font-semibold text-muted uppercase">
              {user?.full_name?.slice(0, 1) ?? '?'}
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] leading-tight font-medium">{user?.full_name ?? '-'}</p>
              <p className="truncate text-[11px] leading-tight text-muted">
                {user?.roles.join(', ') || 'Tanpa role'}
              </p>
            </div>
            <button
              type="button"
              onClick={handleLogout}
              className="grid size-11 shrink-0 place-items-center rounded-control text-muted hover:bg-raised hover:text-ink"
              aria-label="Keluar"
              title="Keluar"
            >
              <LogOut className="size-4" strokeWidth={1.5} />
            </button>
          </div>
        </div>
      </aside>
    </>
  )
}
