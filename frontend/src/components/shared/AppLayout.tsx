import { Menu } from 'lucide-react'
import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'

export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false)

  return (
    <div className="min-h-dvh bg-ground">
      <Sidebar open={navOpen} onClose={() => setNavOpen(false)} />

      <div className="lg:pl-57">
        <div className="flex h-12 items-center gap-1 px-3 lg:hidden">
          <button
            type="button"
            className="grid size-11 place-items-center rounded-control text-muted hover:bg-raised"
            onClick={() => setNavOpen(true)}
            aria-label="Buka menu"
          >
            <Menu className="size-5" strokeWidth={1.5} />
          </button>
          <span className="text-sm font-semibold">Food POS</span>
        </div>

        {/* Lembar kerja: putih, inset 8px dari kanan dan bawah, sudut 16, tanpa bayangan. */}
        <main className="pb-2 lg:pr-2">
          <div className="min-h-[calc(100dvh-3rem)] rounded-sheet border border-line bg-surface px-4 py-5 lg:min-h-[calc(100dvh-8px)] lg:px-6 lg:py-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
