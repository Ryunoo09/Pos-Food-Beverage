import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { rolesApi, usersApi } from '../../api/users'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'

const PER_PAGE = 20

export function UsersPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('users.create')
  const canToggle = permissions.includes('users.update') || permissions.includes('users.deactivate')

  const [page, setPage] = useState(1)
  const [createOpen, setCreateOpen] = useState(false)

  const users = useQuery({
    queryKey: ['users', page],
    queryFn: async () => (await usersApi.list({ page, per_page: PER_PAGE })).data,
  })
  const roles = useQuery({
    queryKey: ['roles'],
    queryFn: async () => (await rolesApi.list()).data.data,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['users'] })

  const create = useMutation({
    mutationFn: (body: Parameters<typeof usersApi.create>[0]) => usersApi.create(body),
    onSuccess: async () => {
      setCreateOpen(false)
      await invalidate()
      toast.push('Pengguna ditambahkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const toggle = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      active ? usersApi.activate(id) : usersApi.deactivate(id),
    onSuccess: async () => {
      await invalidate()
      toast.push('Status pengguna diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const rows = users.data?.data ?? []
  const meta = users.data?.meta

  return (
    <>
      <PageHeader
        title="Pengguna"
        description="Akun yang bisa masuk ke sistem ini beserta perannya."
        action={
          canCreate && (
            <Button size="sm" variant="secondary" onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" strokeWidth={2} />
              Tambah
            </Button>
          )
        }
      />

      <Card className="mt-6">
        <CardHeader title="Daftar pengguna" />
        {users.isLoading ? (
          <div className="space-y-3 p-5">
            {Array.from({ length: 5 }, (_, index) => (
              <Skeleton key={index} className="h-10 w-full" />
            ))}
          </div>
        ) : users.isError ? (
          <p className="px-5 py-8 text-sm text-negative">{apiMessage(users.error)}</p>
        ) : rows.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-muted">Belum ada pengguna.</p>
        ) : (
          <ul className="divide-y divide-rule">
            {rows.map((user) => (
              <li key={user.id} className="flex flex-wrap items-center gap-3 px-5 py-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{user.full_name}</p>
                    <span className="num text-xs text-muted">{user.username}</span>
                    {!user.is_active && <Badge tone="gray">Nonaktif</Badge>}
                    {user.must_change_password && <Badge tone="yellow">Ganti sandi</Badge>}
                  </div>
                  <p className="mt-0.5 text-xs text-muted">
                    {roleLabels(user.roles, roles.data ?? []).join(' · ') || 'Tanpa peran'}
                    {user.email ? ` · ${user.email}` : ''}
                  </p>
                </div>
                {canToggle && (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={toggle.isPending}
                    onClick={() => toggle.mutate({ id: user.id, active: !user.is_active })}
                  >
                    {user.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {meta && (
          <Pagination
            page={meta.page}
            totalPages={meta.total_pages}
            total={meta.total}
            onPageChange={setPage}
          />
        )}
      </Card>

      {createOpen && (
        <CreateUserDialog
          loading={create.isPending}
          roles={roles.data ?? []}
          onOpenChange={(open) => !open && setCreateOpen(false)}
          onSubmit={(body) => create.mutate(body)}
        />
      )}
    </>
  )
}

/** Peran disimpan sebagai kode; tampilkan nama yang bisa dibaca bila ada. */
function roleLabels(codes: string[], roles: { code: string; name: string }[]): string[] {
  return codes.map((code) => roles.find((role) => role.code === code)?.name ?? code)
}

function CreateUserDialog({
  loading,
  roles,
  onOpenChange,
  onSubmit,
}: {
  loading: boolean
  roles: { id: string; code: string; name: string }[]
  onOpenChange: (open: boolean) => void
  onSubmit: (body: {
    username: string
    full_name: string
    email: string | null
    password: string
    role_ids: string[]
  }) => void
}) {
  const [username, setUsername] = useState('')
  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [roleIds, setRoleIds] = useState<string[]>([])

  // Sandi minimal 8 karakter, sama dengan aturan validasi backend.
  const passwordValid = password.length >= 8
  const canSubmit =
    username.trim() !== '' && fullName.trim() !== '' && passwordValid && roleIds.length > 0

  return (
    <Dialog
      open
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Tambah pengguna"
      description="Pengguna baru wajib mengganti sandi saat pertama masuk."
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button
            loading={loading}
            disabled={!canSubmit}
            onClick={() =>
              onSubmit({
                username: username.trim(),
                full_name: fullName.trim(),
                email: email.trim() === '' ? null : email.trim(),
                password,
                role_ids: roleIds,
              })
            }
          >
            Simpan
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Input
          label="Nama lengkap"
          value={fullName}
          onChange={(event) => setFullName(event.target.value)}
        />
        <Input
          label="Username"
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          helper="Dipakai untuk masuk. Tidak bisa diubah setelah dibuat."
        />
        <Input
          label="Email (opsional)"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
        <Input
          label="Sandi awal"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          error={password !== '' && !passwordValid ? 'Minimal 8 karakter.' : undefined}
        />
        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">Peran</legend>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {roles.map((role) => (
              <label key={role.id} className="flex min-h-11 items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-brand-600"
                  checked={roleIds.includes(role.id)}
                  onChange={(event) =>
                    setRoleIds((prev) =>
                      event.target.checked
                        ? [...prev, role.id]
                        : prev.filter((id) => id !== role.id),
                    )
                  }
                />
                {role.name}
              </label>
            ))}
          </div>
          <p className="mt-1 text-xs text-muted">Pilih minimal satu peran.</p>
        </fieldset>
      </div>
    </Dialog>
  )
}
