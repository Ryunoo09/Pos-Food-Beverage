import { Link } from 'react-router-dom'

function ErrorScreen({
  code,
  title,
  description,
}: {
  code: string
  title: string
  description: string
}) {
  return (
    <main className="grid min-h-[60vh] place-items-center p-6">
      <div className="max-w-sm">
        <p className="num text-xs text-muted">{code}</p>
        <h1 className="mt-1 text-[22px] leading-tight font-semibold tracking-[-0.01em]">{title}</h1>
        <p className="mt-1.5 text-sm text-muted">{description}</p>
        <Link
          className="mt-5 inline-flex min-h-11 items-center text-sm font-medium text-ink underline underline-offset-4"
          to="/dashboard"
        >
          Kembali ke Dashboard
        </Link>
      </div>
    </main>
  )
}

export function ForbiddenPage() {
  return (
    <ErrorScreen
      code="403"
      title="Akses ditolak"
      description="Akun Anda tidak punya izin membuka halaman ini. Minta pemilik usaha menambah izin pada role Anda."
    />
  )
}

export function NotFoundPage() {
  return (
    <ErrorScreen
      code="404"
      title="Halaman tidak ditemukan"
      description="Alamat ini tidak ada, atau sudah dipindahkan."
    />
  )
}
