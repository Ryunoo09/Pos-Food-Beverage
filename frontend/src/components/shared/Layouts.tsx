import { ArrowLeft } from 'lucide-react'
import { Link, Outlet } from 'react-router-dom'
import { ProductMark } from './ProductMark'

/** Layar kasir: penuh, tanpa sidebar, tombol besar. */
export function POSLayout() {
  return (
    <div className="flex min-h-dvh flex-col bg-ground">
      <header className="flex h-14 items-center gap-3 px-4">
        <Link
          to="/dashboard"
          className="flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
        >
          <ArrowLeft className="size-4" strokeWidth={1.5} />
          Keluar POS
        </Link>
        <span className="ml-auto flex items-center gap-2">
          <ProductMark className="size-5 text-brand-600" />
          <span className="text-sm font-semibold">Kasir</span>
        </span>
      </header>
      <main className="flex flex-1 pr-2 pb-2">
        <div className="flex-1 rounded-sheet border border-line bg-surface p-5">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

/** Layar dapur: kontras tinggi, dibaca dari jarak 1-2 meter. */
export function KDSLayout() {
  return (
    <div className="dark min-h-dvh bg-ground text-ink">
      <header className="flex h-16 items-center justify-between px-6">
        <span className="flex items-center gap-3 text-xl font-semibold tracking-wide">
          <ProductMark className="size-6 text-brand-500" />
          Dapur
        </span>
        <Link
          to="/dashboard"
          className="flex min-h-11 items-center rounded-control px-4 text-base font-medium text-muted hover:bg-raised hover:text-ink"
        >
          Keluar
        </Link>
      </header>
      <main className="px-4 pb-4">
        <div className="min-h-[calc(100dvh-5rem)] rounded-sheet border border-line bg-surface p-6">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
