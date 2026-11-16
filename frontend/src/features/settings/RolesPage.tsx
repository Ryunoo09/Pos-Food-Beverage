import { useQuery } from '@tanstack/react-query'
import { rolesApi } from '../../api/users'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Card, CardHeader } from '../../components/ui/Card'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'

export function RolesPage() {
  const roles = useQuery({
    queryKey: ['roles'],
    queryFn: async () => (await rolesApi.list()).data.data,
  })

  return (
    <>
      <PageHeader
        title="Peran"
        description="Peran menentukan menu dan tombol yang bisa dipakai pengguna. Peran bawaan sistem tidak bisa dihapus."
      />

      <Card className="mt-6">
        <CardHeader title="Daftar peran" />
        {roles.isLoading ? (
          <div className="space-y-3 p-5">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton key={index} className="h-10 w-full" />
            ))}
          </div>
        ) : roles.isError ? (
          <p className="px-5 py-8 text-sm text-negative">{apiMessage(roles.error)}</p>
        ) : (roles.data ?? []).length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-muted">Belum ada peran.</p>
        ) : (
          <ul className="divide-y divide-rule">
            {(roles.data ?? []).map((role) => (
              <li key={role.id} className="flex flex-wrap items-start gap-3 px-5 py-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{role.name}</p>
                    <span className="num text-xs text-muted">{role.code}</span>
                    {role.is_system && <Badge tone="blue">Bawaan sistem</Badge>}
                  </div>
                  <p className="mt-0.5 text-xs text-muted">
                    {(role.permissions ?? []).length > 0
                      ? `${(role.permissions ?? []).length} izin`
                      : 'Rincian izin tidak tersedia'}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </>
  )
}
