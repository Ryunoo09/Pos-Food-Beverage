import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { settingsApi } from '../../api/settings'
import type { PaymentMethod } from '../../api/settings'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'

export function PaymentMethodsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canManage = permissions.includes('settings.manage')

  const [createOpen, setCreateOpen] = useState(false)

  const methods = useQuery({
    queryKey: ['settings-payment-methods'],
    queryFn: async () => (await settingsApi.paymentMethods.list()).data.data,
  })

  const create = useMutation({
    mutationFn: (body: Parameters<typeof settingsApi.paymentMethods.create>[0]) =>
      settingsApi.paymentMethods.create(body),
    onSuccess: async () => {
      setCreateOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['settings-payment-methods'] })
      toast.push('Metode bayar ditambahkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const toggle = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      active
        ? settingsApi.paymentMethods.activate(id)
        : settingsApi.paymentMethods.deactivate(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['settings-payment-methods'] })
      toast.push('Metode bayar diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  return (
    <>
      <PageHeader
        title="Metode pembayaran"
        description="Cara bayar yang bisa dipilih kasir. Kode dipakai di transaksi dan laporan, jadi tidak bisa diubah setelah dipakai."
        action={
          canManage && (
            <Button size="sm" variant="secondary" onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" strokeWidth={2} />
              Tambah
            </Button>
          )
        }
      />

      <Card className="mt-6">
        <CardHeader title="Daftar metode" />
        {methods.isLoading ? (
          <div className="space-y-3 p-5">
            {Array.from({ length: 4 }, (_, index) => (
              <Skeleton key={index} className="h-10 w-full" />
            ))}
          </div>
        ) : methods.isError ? (
          <p className="px-5 py-8 text-sm text-negative">{apiMessage(methods.error)}</p>
        ) : (methods.data ?? []).length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-muted">Belum ada metode bayar.</p>
        ) : (
          <ul className="divide-y divide-rule">
            {(methods.data ?? []).map((method) => (
              <li key={method.id} className="flex flex-wrap items-center gap-3 px-5 py-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="num text-[13px] text-ink">{method.code}</span>
                    <p className="text-[13px]">{method.name}</p>
                    {method.is_cash && <Badge tone="green">Kas laci</Badge>}
                    {method.requires_reference && <Badge tone="yellow">Butuh referensi</Badge>}
                    {!method.is_active && <Badge tone="gray">Nonaktif</Badge>}
                  </div>
                  <p className="mt-0.5 text-xs text-muted">
                    {method.provider !== '' ? `Penyedia: ${method.provider}` : 'Tanpa penyedia'}
                    {` · urutan ${method.sort_order}`}
                  </p>
                </div>
                {canManage && (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={toggle.isPending}
                    onClick={() => toggle.mutate({ id: method.id, active: !method.is_active })}
                  >
                    {method.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Card>

      {createOpen && (
        <CreatePaymentMethodDialog
          loading={create.isPending}
          existing={methods.data ?? []}
          onOpenChange={(open) => !open && setCreateOpen(false)}
          onSubmit={(body) => create.mutate(body)}
        />
      )}
    </>
  )
}

function CreatePaymentMethodDialog({
  loading,
  existing,
  onOpenChange,
  onSubmit,
}: {
  loading: boolean
  existing: PaymentMethod[]
  onOpenChange: (open: boolean) => void
  onSubmit: (body: {
    code: string
    name: string
    requires_reference: boolean
    provider: string | null
    sort_order: number
  }) => void
}) {
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [requiresReference, setRequiresReference] = useState(false)
  const [provider, setProvider] = useState('')

  const normalized = code.trim().toUpperCase()
  const codeValid = /^[A-Z0-9_]+$/.test(normalized)
  const duplicate = existing.some((method) => method.code === normalized)
  const canSubmit = codeValid && !duplicate && name.trim() !== ''

  return (
    <Dialog
      open
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Tambah metode bayar"
      description="Kode ditulis huruf besar dan tidak bisa diubah setelah dipakai transaksi."
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
                code: normalized,
                name: name.trim(),
                requires_reference: requiresReference,
                provider: provider.trim() === '' ? null : provider.trim(),
                sort_order: existing.length + 1,
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
          label="Kode"
          value={code}
          onChange={(event) => setCode(event.target.value.toUpperCase())}
          placeholder="DEBIT"
          error={
            code !== '' && !codeValid
              ? 'Hanya huruf, angka, dan garis bawah.'
              : duplicate
                ? 'Kode sudah dipakai.'
                : undefined
          }
        />
        <Input
          label="Nama"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Kartu debit"
        />
        <Input
          label="Penyedia (opsional)"
          value={provider}
          onChange={(event) => setProvider(event.target.value)}
          helper="Mis. nama bank atau penyedia EDC. Boleh dikosongkan."
        />
        <label className="flex min-h-11 items-center gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="size-4 accent-brand-600"
            checked={requiresReference}
            onChange={(event) => setRequiresReference(event.target.checked)}
          />
          Kasir wajib mengisi nomor referensi
        </label>
      </div>
    </Dialog>
  )
}
