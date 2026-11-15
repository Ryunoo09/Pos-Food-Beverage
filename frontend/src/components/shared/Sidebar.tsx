import { LogOut, Menu, X } from 'lucide-react'
import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../store/auth'
import { cn } from '../../utils/cn'
import { navGroups } from './nav'

export function Sidebar() {
  const [open, setOpen] = useState(false)
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
      <button
        type="button"
        className="fixed top-3 left-3 z-40 grid size-11 place-items-center rounded-lg bg-white shadow lg:hidden dark:bg-slate-900"
        onClick={() => setOpen(true)}
        aria-label="Buka menu"
      >
        <Menu className="size-5" />
      </button>

      {open && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/40 lg:hidden"
          onClick={() => setOpen(false)}
          aria-hidden
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-slate-200 bg-white transition-transform dark:border-slate-800 dark:bg-slate-900',
          'lg:translate-x-0',
          open ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-16 items-center justify-between px-4">
          <span className="text-lg font-semibold text-brand-600">Food POS</span>
          <button
            type="button"
            className="grid size-11 place-items-center rounded-lg hover:bg-slate-100 lg:hidden dark:hover:bg-slate-800"
            onClick={() => setOpen(false)}
            aria-label="Tutup menu"
          >
            <X className="size-5" />
          </button>
        </div>

        <nav className="flex-1 space-y-6 overflow-y-auto px-3 pb-4">
          {navGroups.map((group) => {
            const items = group.items.filter((item) => permissions.includes(item.permission))
            if (items.length === 0) return null
            return (
              <div key={group.label}>
                <p className="px-3 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase">
                  {group.label}
                </p>
                <ul className="space-y-1">
                  {items.map(({ label, to, icon: Icon }) => (
                    <li key={to}>
                      <NavLink
                        to={to}
                        onClick={() => setOpen(false)}
                        className={({ isActive }) =>
                          cn(
                            'flex min-h-11 items-center gap-3 rounded-lg px-3 text-sm font-medium',
                            isActive
                              ? 'bg-brand-50 text-brand-700 dark:bg-brand-700/20 dark:text-brand-100'
                              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800',
                          )
                        }
                      >
                        <Icon className="size-4 shrink-0" />
                        {label}
                      </NavLink>
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </nav>

        <div className="border-t border-slate-200 p-3 dark:border-slate-800">
          <p className="truncate px-3 text-sm font-medium">{user?.full_name ?? '-'}</p>
          <p className="truncate px-3 pb-2 text-xs text-slate-500">
            {user?.roles.join(', ') || 'Tanpa role'}
          </p>
          <button
            type="button"
            onClick={handleLogout}
            className="flex min-h-11 w-full items-center gap-3 rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            <LogOut className="size-4" />
            Keluar
          </button>
        </div>
      </aside>
    </>
  )
}
