import { Navigate, createBrowserRouter } from 'react-router-dom'
import { AppLayout } from './components/shared/AppLayout'
import { KDSLayout, POSLayout } from './components/shared/Layouts'
import { PermissionGate } from './components/shared/PermissionGate'
import { RequireAuth } from './components/shared/RequireAuth'
import { ChangePasswordPage } from './features/auth/ChangePasswordPage'
import { LoginPage } from './features/auth/LoginPage'
import { DashboardPage } from './features/dashboard/DashboardPage'
import { ForbiddenPage, NotFoundPage } from './features/errors/ErrorPages'
import { ExpensesPage } from './features/expenses/ExpensesPage'
import { IngredientDetailPage } from './features/inventory/IngredientDetailPage'
import { IngredientsPage } from './features/inventory/IngredientsPage'
import { InventoryPage } from './features/inventory/InventoryPage'
import { OpnameDetailPage } from './features/inventory/OpnameDetailPage'
import { OpnamePage } from './features/inventory/OpnamePage'
import { KitchenPage } from './features/kitchen/KitchenPage'
import { CategoriesPage } from './features/menu/CategoriesPage'
import { MenuListPage } from './features/menu/MenuListPage'
import { ProductDetailPage } from './features/menu/ProductDetailPage'
import { RecipePage } from './features/menu/RecipePage'
import { OrderDetailPage } from './features/orders/OrderDetailPage'
import { OrdersPage } from './features/orders/OrdersPage'
import { PosPage } from './features/pos/PosPage'
import { PurchaseDetailPage } from './features/purchases/PurchaseDetailPage'
import { PurchasePage } from './features/purchases/PurchasePage'
import { FinancialReportPage } from './features/reports/FinancialReportPage'
import { InventoryReportPage } from './features/reports/InventoryReportPage'
import { MenuPerformanceReportPage } from './features/reports/MenuPerformanceReportPage'
import { SalesReportPage } from './features/reports/SalesReportPage'
import { ShiftReportPage } from './features/reports/ShiftReportPage'
import { TaxSettingsPage } from './features/settings/TaxSettingsPage'
import { SettingsPage } from './features/settings/SettingsPage'
import { UsersPage } from './features/settings/UsersPage'
import { RolesPage } from './features/settings/RolesPage'
import { PaymentMethodsPage } from './features/settings/PaymentMethodsPage'
import { AuditLogPage } from './features/audit/AuditLogPage'
import { ShiftDetailPage } from './features/shifts/ShiftDetailPage'
import { ShiftsPage } from './features/shifts/ShiftsPage'
import { SuppliersPage } from './features/suppliers/SuppliersPage'
import { TablesPage } from './features/tables/TablesPage'
import { WastePage } from './features/waste/WastePage'

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
          {
            path: '/recipes/:variantId',
            element: (
              <PermissionGate perm="recipes.view" redirect="/403">
                <RecipePage />
              </PermissionGate>
            ),
          },
          {
            path: '/inventory',
            element: (
              <PermissionGate perm="inventory.view" redirect="/403">
                <InventoryPage />
              </PermissionGate>
            ),
          },
          {
            path: '/inventory/opname',
            element: (
              <PermissionGate perm="stock_opname.view" redirect="/403">
                <OpnamePage />
              </PermissionGate>
            ),
          },
          {
            path: '/inventory/opname/:id',
            element: (
              <PermissionGate perm="stock_opname.view" redirect="/403">
                <OpnameDetailPage />
              </PermissionGate>
            ),
          },
          {
            path: '/purchase',
            element: (
              <PermissionGate perm="purchases.view" redirect="/403">
                <PurchasePage />
              </PermissionGate>
            ),
          },
          {
            path: '/purchase/:id',
            element: (
              <PermissionGate perm="purchases.view" redirect="/403">
                <PurchaseDetailPage />
              </PermissionGate>
            ),
          },
          { path: '/suppliers', element: <SuppliersPage /> },
          {
            path: '/waste',
            element: (
              <PermissionGate perm="waste.view" redirect="/403">
                <WastePage />
              </PermissionGate>
            ),
          },
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
          {
            path: '/expenses',
            element: (
              <PermissionGate perm="expenses.view" redirect="/403">
                <ExpensesPage />
              </PermissionGate>
            ),
          },
          { path: '/reports', element: <Navigate to="/reports/sales" replace /> },
          {
            path: '/reports/sales',
            element: (
              <PermissionGate perm="reports.sales.view" redirect="/403">
                <SalesReportPage />
              </PermissionGate>
            ),
          },
          {
            path: '/reports/financial',
            element: (
              <PermissionGate perm="reports.profit.view" redirect="/403">
                <FinancialReportPage />
              </PermissionGate>
            ),
          },
          {
            path: '/reports/inventory',
            element: (
              <PermissionGate perm="reports.inventory.view" redirect="/403">
                <InventoryReportPage />
              </PermissionGate>
            ),
          },
          {
            path: '/reports/menu-performance',
            element: (
              <PermissionGate perm="reports.menu_performance.view" redirect="/403">
                <MenuPerformanceReportPage />
              </PermissionGate>
            ),
          },
          {
            path: '/reports/shifts',
            element: (
              <PermissionGate perm="reports.shift.view" redirect="/403">
                <ShiftReportPage />
              </PermissionGate>
            ),
          },
          {
            path: '/settings',
            element: (
              <PermissionGate perm="settings.view" redirect="/403">
                <SettingsPage />
              </PermissionGate>
            ),
          },
          {
            path: '/settings/users',
            element: (
              <PermissionGate perm="users.view" redirect="/403">
                <UsersPage />
              </PermissionGate>
            ),
          },
          {
            path: '/settings/roles',
            element: (
              <PermissionGate perm="roles.view" redirect="/403">
                <RolesPage />
              </PermissionGate>
            ),
          },
          {
            path: '/settings/tax',
            element: (
              <PermissionGate perm="settings.view" redirect="/403">
                <TaxSettingsPage />
              </PermissionGate>
            ),
          },
          {
            path: '/settings/payment-methods',
            element: (
              <PermissionGate perm="settings.view" redirect="/403">
                <PaymentMethodsPage />
              </PermissionGate>
            ),
          },
          {
            path: '/audit-logs',
            element: (
              <PermissionGate perm="audit.view" redirect="/403">
                <AuditLogPage />
              </PermissionGate>
            ),
          },
          { path: '/403', element: <ForbiddenPage /> },
          { path: '*', element: <NotFoundPage /> },
        ],
      },
    ],
  },
])
