import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AppLayout } from './components/shared/AppLayout'
import { KDSLayout, POSLayout } from './components/shared/Layouts'
import { PermissionGate } from './components/shared/PermissionGate'
import { RequireAuth } from './components/shared/RequireAuth'
import { ChangePasswordPage } from './features/auth/ChangePasswordPage'
import { LoginPage } from './features/auth/LoginPage'
import { DashboardPage } from './features/dashboard/DashboardPage'
import { ForbiddenPage, NotFoundPage } from './features/errors/ErrorPages'
import { IngredientDetailPage } from './features/inventory/IngredientDetailPage'
import { IngredientsPage } from './features/inventory/IngredientsPage'
import { KitchenPage } from './features/kitchen/KitchenPage'
import { CategoriesPage } from './features/menu/CategoriesPage'
import { MenuListPage } from './features/menu/MenuListPage'
import { ProductDetailPage } from './features/menu/ProductDetailPage'
import { OrderDetailPage } from './features/orders/OrderDetailPage'
import { OrdersPage } from './features/orders/OrdersPage'
import { PosPage } from './features/pos/PosPage'
import { PlaceholderPage } from './features/shared/PlaceholderPage'
import { ShiftDetailPage } from './features/shifts/ShiftDetailPage'
import { ShiftsPage } from './features/shifts/ShiftsPage'
import { SuppliersPage } from './features/suppliers/SuppliersPage'
import { TablesPage } from './features/tables/TablesPage'

function page(title: string, permission: string, description?: string) {
  return (
    <PermissionGate perm={permission} redirect="/403">
      <PlaceholderPage title={title} permission={permission} description={description} />
    </PermissionGate>
  )
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth />,
    children: [
      { path: '/change-password', element: <ChangePasswordPage /> },
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      {
        path: '/pos',
        element: <POSLayout />,
        children: [
          {
            index: true,
            element: (
              <PermissionGate perm="orders.create" redirect="/403">
                <PosPage />
              </PermissionGate>
            ),
          },
          { path: '*', element: <NotFoundPage /> },
        ],
      },
      {
        path: '/kitchen',
        element: <KDSLayout />,
        children: [
          {
            index: true,
            element: (
              <PermissionGate perm="kitchen.orders.view" redirect="/403">
                <KitchenPage />
              </PermissionGate>
            ),
          },
          { path: '*', element: <NotFoundPage /> },
        ],
      },
      {
        element: <AppLayout />,
        children: [
          { path: '/dashboard', element: <DashboardPage /> },
          {
            path: '/orders',
            element: (
              <PermissionGate perm="orders.view" redirect="/403">
                <OrdersPage />
              </PermissionGate>
            ),
          },
          {
            path: '/orders/:id',
            element: (
              <PermissionGate perm="orders.view" redirect="/403">
                <OrderDetailPage />
              </PermissionGate>
            ),
          },
          { path: '/tables', element: <TablesPage /> },
          { path: '/menu', element: <MenuListPage /> },
          { path: '/menu/categories', element: <CategoriesPage /> },
          { path: '/menu/:id', element: <ProductDetailPage /> },
          { path: '/ingredients', element: <IngredientsPage /> },
          { path: '/ingredients/:id', element: <IngredientDetailPage /> },
          { path: '/recipes/:variantId', element: page('Resep', 'recipes.view') },
          { path: '/inventory', element: page('Stok', 'inventory.view') },
          { path: '/inventory/opname', element: page('Stock Opname', 'stock_opname.view') },
          { path: '/inventory/opname/:id', element: page('Detail Opname', 'stock_opname.view') },
          { path: '/purchase', element: page('Pembelian', 'purchases.view') },
          { path: '/purchase/:id', element: page('Detail Pembelian', 'purchases.view') },
          { path: '/suppliers', element: <SuppliersPage /> },
          { path: '/waste', element: page('Waste', 'waste.view') },
          {
            path: '/shifts',
            element: (
              <PermissionGate perm="shifts.view_all" redirect="/403">
                <ShiftsPage />
              </PermissionGate>
            ),
          },
          {
            path: '/shifts/:id',
            element: (
              <PermissionGate perm="shifts.view" redirect="/403">
                <ShiftDetailPage />
              </PermissionGate>
            ),
          },
          { path: '/expenses', element: page('Pengeluaran', 'expenses.view') },
          { path: '/reports', element: page('Laporan', 'reports.sales.view') },
          { path: '/reports/sales', element: page('Laporan Penjualan', 'reports.sales.view') },
          { path: '/reports/financial', element: page('Laporan Keuangan', 'reports.profit.view') },
          { path: '/reports/inventory', element: page('Laporan Inventori', 'reports.inventory.view') },
          {
            path: '/reports/menu-performance',
            element: page('Performa Menu', 'reports.menu_performance.view'),
          },
          { path: '/settings', element: page('Pengaturan', 'settings.view') },
          { path: '/settings/users', element: page('Pengguna', 'users.view') },
          { path: '/settings/roles', element: page('Role', 'roles.view') },
          { path: '/settings/tax', element: page('Pajak', 'tax.manage') },
          { path: '/settings/payment-methods', element: page('Metode Pembayaran', 'settings.view') },
          { path: '/403', element: <ForbiddenPage /> },
          { path: '*', element: <NotFoundPage /> },
        ],
      },
    ],
  },
])
