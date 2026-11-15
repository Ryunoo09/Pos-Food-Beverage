import { Link, Outlet } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'

/** Layar kasir: penuh, tanpa sidebar, tombol besar. */
export function POSLayout() {
  return (
    <div className="flex min-h-dvh flex-col bg-slate-100 dark:bg-slate-950">
      <header className="flex h-16 items-center gap-4 border-b border-slate-200 bg-white px-4 dark:border-slate-800 dark:bg-slate-900">
        <Link
          to="/dashboard"
          className="flex min-h-11 items-center gap-2 rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <ArrowLeft className="size-4" />
          Keluar POS
        </Link>
        <span className="text-lg font-semibold text-brand-600">Kasir</span>
      </header>
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  )
}

/** Layar dapur: kontras tinggi, dibaca dari jarak 1-2 meter. */
export function KDSLayout() {
  return (
    <div className="min-h-dvh bg-slate-950 text-slate-50">
      <header className="flex h-16 items-center justify-between border-b border-slate-800 px-6">
        <span className="text-xl font-bold tracking-wide">DAPUR</span>
        <Link
          to="/dashboard"
          className="flex min-h-11 items-center rounded-lg px-4 text-base font-semibold text-slate-300 hover:bg-slate-800"
        >
          Keluar
        </Link>
      </header>
      <main className="p-6">
        <Outlet />
      </main>
    </div>
  )
}
