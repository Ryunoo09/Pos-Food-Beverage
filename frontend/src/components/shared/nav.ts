import {
  BarChart3,
  Boxes,
  ChefHat,
  ClipboardList,
  CreditCard,
  LayoutDashboard,
  Package,
  Receipt,
  Settings,
  ShieldCheck,
  ShoppingCart,
  Store,
  Table2,
  Trash2,
  Truck,
  Users,
  UtensilsCrossed,
  Wallet,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface NavItem {
  label: string
  to: string
  permission: string
  icon: LucideIcon
}

export interface NavGroup {
  label: string
  items: NavItem[]
}

// Permission per item = kode yang diperiksa backend; UI hanya menyembunyikan
// menu, penolakan tetap terjadi di server.
export const navGroups: NavGroup[] = [
  {
    label: 'Utama',
    items: [
      { label: 'Dashboard', to: '/dashboard', permission: 'dashboard.view', icon: LayoutDashboard },
      { label: 'POS', to: '/pos', permission: 'orders.create', icon: ShoppingCart },
      { label: 'Pesanan', to: '/orders', permission: 'orders.view', icon: ClipboardList },
      { label: 'Meja', to: '/tables', permission: 'tables.view', icon: Table2 },
      { label: 'Dapur', to: '/kitchen', permission: 'kitchen.orders.view', icon: ChefHat },
    ],
  },
  {
    label: 'Menu',
    items: [
      { label: 'Menu', to: '/menu', permission: 'products.view', icon: UtensilsCrossed },
      { label: 'Kategori', to: '/menu/categories', permission: 'products.view', icon: Store },
    ],
  },
  {
    label: 'Inventori',
    items: [
      { label: 'Bahan', to: '/ingredients', permission: 'ingredients.view', icon: Boxes },
      { label: 'Resep', to: '/recipes', permission: 'recipes.view', icon: Receipt },
      { label: 'Stok', to: '/inventory', permission: 'inventory.view', icon: Package },
      { label: 'Opname', to: '/inventory/opname', permission: 'stock_opname.view', icon: ClipboardList },
      { label: 'Pembelian', to: '/purchase', permission: 'purchases.view', icon: Truck },
      { label: 'Supplier', to: '/suppliers', permission: 'suppliers.view', icon: Truck },
      { label: 'Waste', to: '/waste', permission: 'waste.view', icon: Trash2 },
    ],
  },
  {
    label: 'Operasional',
    items: [
      { label: 'Shift', to: '/shifts', permission: 'shifts.view', icon: Wallet },
      { label: 'Pengeluaran', to: '/expenses', permission: 'expenses.view', icon: CreditCard },
      { label: 'Laporan', to: '/reports', permission: 'reports.sales.view', icon: BarChart3 },
    ],
  },
  {
    label: 'Pengaturan',
    items: [
      { label: 'Pengguna', to: '/settings/users', permission: 'users.view', icon: Users },
      { label: 'Role', to: '/settings/roles', permission: 'roles.view', icon: ShieldCheck },
      { label: 'Pajak', to: '/settings/tax', permission: 'tax.manage', icon: Settings },
      {
        label: 'Metode Bayar',
        to: '/settings/payment-methods',
        permission: 'settings.view',
        icon: CreditCard,
      },
    ],
  },
]
