import { createBrowserRouter, Link, Navigate } from 'react-router-dom'
import { LoginPage } from './features/auth/LoginPage'
import { DashboardPage } from './features/dashboard/DashboardPage'

export const router = createBrowserRouter([
  { path: '/', element: <Navigate to="/login" replace /> },
  { path: '/login', element: <LoginPage /> },
  { path: '/dashboard', element: <DashboardPage /> },
  {
    path: '*',
    element: (
      <main className="flex min-h-dvh flex-col items-center justify-center gap-4 bg-slate-100 p-4">
        <h1 className="text-2xl font-semibold text-slate-900">Halaman tidak ditemukan</h1>
        <Link className="text-orange-600 underline" to="/dashboard">
          Kembali ke Dashboard
        </Link>
      </main>
    ),
  },
])
