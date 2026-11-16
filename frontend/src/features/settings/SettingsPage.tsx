import { Link } from 'react-router'
import { PageHeader } from '../../components/shared/PageHeader'
import { Card, CardHeader } from '../../components/ui/Card'
import { useAuthStore } from '../../store/auth'

interface Entry {
  to: string
  title: string
  description: string
  perm: string
}

const ENTRIES: Entry[] = [
  {
    to: '/settings/tax',
    title: 'Pajak & diskon',
    description: 'Aturan transaksi, tarif pajak, service charge, dan promo.',
    perm: 'settings.view',
  },
  {
    to: '/settings/payment-methods',
    title: 'Metode pembayaran',
    description: 'Cara bayar yang bisa dipilih kasir.',
    perm: 'settings.view',
  },
  {
    to: '/settings/users',
    title: 'Pengguna',
    description: 'Akun yang bisa masuk dan perannya.',
    perm: 'users.view',
  },
  {
    to: '/settings/roles',
    title: 'Peran',
    description: 'Izin yang dimiliki tiap peran.',
    perm: 'roles.view',
  },
  {
    to: '/audit-logs',
    title: 'Log audit',
    description: 'Catatan perubahan data penting.',
    perm: 'audit.view',
  },
]

export function SettingsPage() {
  const permissions = useAuthStore((state) => state.permissions)
  // Hanya tampilkan pintasan yang memang boleh dibuka pengguna ini.
  const visible = ENTRIES.filter((entry) => permissions.includes(entry.perm))

  return (
    <>
      <PageHeader
        title="Pengaturan"
        description="Atur aturan transaksi, pengguna, dan jejak audit outlet ini."
      />

      {visible.length === 0 ? (
        <p className="mt-6 text-sm text-muted">Tidak ada pengaturan yang bisa diakses peran Anda.</p>
      ) : (
        <Card className="mt-6">
          <CardHeader title="Bagian pengaturan" />
          <ul className="divide-y divide-rule">
            {visible.map((entry) => (
              <li key={entry.to}>
                <Link
                  to={entry.to}
                  className="flex min-h-11 flex-col justify-center gap-0.5 px-5 py-3 hover:bg-raised"
                >
                  <span className="text-sm font-medium text-ink">{entry.title}</span>
                  <span className="text-xs text-muted">{entry.description}</span>
                </Link>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </>
  )
}
