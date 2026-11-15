import { Link } from 'react-router-dom'

export function ForbiddenPage() {
  return (
    <main className="grid min-h-[60vh] place-items-center p-6 text-center">
      <div>
        <p className="text-5xl font-bold text-brand-600">403</p>
        <h1 className="mt-2 text-xl font-semibold">Akses ditolak</h1>
        <p className="mt-1 text-slate-600 dark:text-slate-300">
          Akun Anda tidak punya izin membuka halaman ini.
        </p>
        <Link className="mt-6 inline-flex min-h-11 items-center text-brand-600 underline" to="/dashboard">
          Kembali ke Dashboard
        </Link>
      </div>
    </main>
  )
}

export function NotFoundPage() {
  return (
    <main className="grid min-h-[60vh] place-items-center p-6 text-center">
      <div>
        <p className="text-5xl font-bold text-brand-600">404</p>
        <h1 className="mt-2 text-xl font-semibold">Halaman tidak ditemukan</h1>
        <Link className="mt-6 inline-flex min-h-11 items-center text-brand-600 underline" to="/dashboard">
          Kembali ke Dashboard
        </Link>
      </div>
    </main>
  )
}
