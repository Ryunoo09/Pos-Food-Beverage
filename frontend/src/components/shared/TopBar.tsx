import { useLocation } from 'react-router-dom'

const LABELS: Record<string, string> = {
  dashboard: 'Dashboard',
  pos: 'POS',
  orders: 'Pesanan',
  kitchen: 'Dapur',
  tables: 'Meja',
  menu: 'Menu',
  categories: 'Kategori',
  ingredients: 'Bahan',
  recipes: 'Resep',
  inventory: 'Stok',
  opname: 'Opname',
  purchase: 'Pembelian',
  suppliers: 'Supplier',
  waste: 'Waste',
  shifts: 'Shift',
  expenses: 'Pengeluaran',
  reports: 'Laporan',
  settings: 'Pengaturan',
  users: 'Pengguna',
  roles: 'Role',
  tax: 'Pajak',
  'payment-methods': 'Metode Bayar',
}

export function TopBar() {
  const { pathname } = useLocation()
  const segments = pathname.split('/').filter(Boolean)

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between gap-4 border-b border-slate-200 bg-white/90 px-4 pl-16 backdrop-blur lg:pl-6 dark:border-slate-800 dark:bg-slate-900/90">
      <nav aria-label="Breadcrumb" className="min-w-0">
        <ol className="flex items-center gap-2 truncate text-sm text-slate-500">
          {segments.map((segment, index) => (
            <li key={`${segment}-${index}`} className="truncate">
              {index > 0 && <span className="pr-2 text-slate-300">/</span>}
              <span className={index === segments.length - 1 ? 'font-medium text-slate-900 dark:text-slate-100' : ''}>
                {LABELS[segment] ?? segment}
              </span>
            </li>
          ))}
        </ol>
      </nav>
    </header>
  )
}
