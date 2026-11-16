import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Check, Play, Send, X } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { OPNAME_STATUS_LABELS, opnameApi } from '../../api/opname'
import type { OpnameItem } from '../../api/opname'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { opnameStatusTone } from '../../utils/status'

export function OpnameDetailPage() {
  const { id = '' } = useParams()
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const userID = useAuthStore((state) => state.user?.id)

  // Draf hitung hanya menyimpan perubahan lokal; nilai awal dibaca langsung dari
  // data server saat render sehingga tidak perlu sinkronisasi di effect.
  const [countEdits, setCountEdits] = useState<Record<string, { quantity: string; reason: string }>>({})
  const [rejectOpen, setRejectOpen] = useState(false)
  const [rejectReason, setRejectReason] = useState('')

  const opname = useQuery({
    queryKey: ['opname', id],
    queryFn: async () => (await opnameApi.get(id)).data.data,
    enabled: id !== '',
  })

  const data = opname.data
  const status = data?.status

  function countValue(item: OpnameItem) {
    return (
      countEdits[item.id] ?? {
        quantity: item.counted_quantity ?? '',
        reason: item.reason ?? '',
      }
    )
  }

  function setCountValue(item: OpnameItem, value: { quantity: string; reason: string }) {
    setCountEdits((current) => ({ ...current, [item.id]: value }))
  }

  function invalidate() {
    return Promise.all([
      queryClient.invalidateQueries({ queryKey: ['opname', id] }),
      queryClient.invalidateQueries({ queryKey: ['opnames'] }),
      queryClient.invalidateQueries({ queryKey: ['stock'] }),
    ])
  }

  const action = useMutation({
    mutationFn: async (kind: 'start' | 'submit' | 'approve' | 'cancel' | 'reject') => {
      switch (kind) {
        case 'start':
          return opnameApi.start(id)
        case 'submit':
          return opnameApi.submit(id)
        case 'approve':
          return opnameApi.approve(id)
        case 'cancel':
          return opnameApi.cancel(id)
        default:
          return opnameApi.reject(id, rejectReason.trim())
      }
    },
    onSuccess: async (_result, kind) => {
      if (kind === 'reject') {
        setRejectOpen(false)
        setRejectReason('')
      }
      await invalidate()
      toast.push(ACTION_MESSAGES[kind])
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  // `data` sudah tersedia di sini karena loading/error sudah ditangani di atas.
  const saveCounts = useMutation({
    mutationFn: () => {
      const items = (opname.data?.items ?? [])
        .map((item) => ({ item, value: countValue(item) }))
        .filter(({ value }) => value.quantity.trim() !== '')
        .map(({ item, value }) => ({
          ingredient_id: item.ingredient_id,
          counted_quantity: value.quantity.trim(),
          reason: value.reason.trim() || null,
        }))
      return opnameApi.count(id, { items })
    },
    onSuccess: async () => {
      await invalidate()
      toast.push('Hasil hitung disimpan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  if (opname.isLoading) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner label="Memuat opname" />
      </div>
    )
  }
  if (opname.isError || !data) {
    return (
      <Card className="p-6 text-sm text-negative">
        {apiMessage(opname.error ?? new Error('Sesi opname tidak ditemukan.'))}
      </Card>
    )
  }

  const items = data.items ?? []
  const waiting = items.filter((item) => countValue(item).quantity.trim() !== '').length
  const canCount = status === 'IN_PROGRESS' && permissions.includes('stock_opname.count')
  // Pemisahan tugas: pengaju tidak boleh menyetujui sesinya sendiri. Backend juga
  // menolak; di sini tombolnya tidak ditawarkan sama sekali.
  const canApprove =
    status === 'SUBMITTED' &&
    permissions.includes('stock_opname.approve') &&
    data.submitted_by !== userID

  return (
    <>
      <Link
        to="/inventory/opname"
        className="inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua sesi opname
      </Link>

      <div className="mt-2">
        <PageHeader
          title={data.code}
          description={`${data.scope === 'ALL' ? 'Semua bahan' : 'Bahan terpilih'} · dibuat ${formatDate(data.created_at)}`}
          action={
            <>
              {status === 'DRAFT' && permissions.includes('stock_opname.create') && (
                <Button loading={action.isPending} onClick={() => action.mutate('start')}>
                  <Play className="size-4" strokeWidth={1.5} />
                  Mulai hitung
                </Button>
              )}
              {canCount && (
                <Button
                  loading={saveCounts.isPending}
                  disabled={waiting === 0}
                  onClick={() => saveCounts.mutate()}
                >
                  <Check className="size-4" strokeWidth={2} />
                  Simpan hasil ({waiting})
                </Button>
              )}
              {status === 'IN_PROGRESS' && permissions.includes('stock_opname.submit') && (
                <Button
                  variant="secondary"
                  loading={action.isPending}
                  onClick={() => action.mutate('submit')}
                >
                  <Send className="size-4" strokeWidth={1.5} />
                  Ajukan
                </Button>
              )}
              {canApprove && (
                <>
                  <Button
                    variant="secondary"
                    onClick={() => setRejectOpen(true)}
                    disabled={action.isPending}
                  >
                    Tolak
                  </Button>
                  <Button loading={action.isPending} onClick={() => action.mutate('approve')}>
                    Setujui
                  </Button>
                </>
              )}
              {(status === 'DRAFT' || status === 'IN_PROGRESS') &&
                permissions.includes('stock_opname.cancel') && (
                  <Button
                    variant="danger"
                    loading={action.isPending}
                    onClick={() => action.mutate('cancel')}
                  >
                    <X className="size-4" strokeWidth={1.5} />
                    Batalkan
                  </Button>
                )}
            </>
          }
        />
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-muted">
        <Badge tone={opnameStatusTone(data.status)}>
          {OPNAME_STATUS_LABELS[data.status] ?? data.status}
        </Badge>
        <span>
          {data.counted_items} dari {data.total_items} bahan dihitung
        </span>
        {data.started_at && <span>· mulai {formatDate(data.started_at)}</span>}
        {data.submitted_at && <span>· diajukan {formatDate(data.submitted_at)}</span>}
        {data.approved_at && <span>· disetujui {formatDate(data.approved_at)}</span>}
      </div>

      {data.rejected_reason && (
        <p className="mt-3 rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
          Ditolak: {data.rejected_reason}
        </p>
      )}

      {status === 'SUBMITTED' && data.submitted_at && (
        <p className="mt-3 text-sm text-muted">
          Sesi menunggu persetujuan.
          {data.submitted_by === userID
            ? ' Anda pengajunya, jadi sesi ini harus disetujui pengguna lain (pemisahan tugas).'
            : ' Pengaju tidak dapat menyetujui sesinya sendiri bila outlet mengaktifkan pemisahan tugas.'}
        </p>
      )}

      <Card className="mt-4">
        <CardHeader
          title="Daftar bahan"
          action={
            canCount && (
              <span className="text-xs text-muted">
                Isi hasil hitung lalu simpan. Kosongkan bila belum dihitung.
              </span>
            )
          }
        />
        {items.length === 0 ? (
          <CardBody className="text-sm text-muted">Sesi ini belum memiliki bahan.</CardBody>
        ) : (
          <ul className="divide-y divide-rule">
            {items.map((item) => (
              <OpnameRow
                key={item.id}
                item={item}
                editable={canCount}
                value={countValue(item)}
                onChange={(next) => setCountValue(item, next)}
              />
            ))}
          </ul>
        )}
      </Card>

      <Dialog
        open={rejectOpen}
        onOpenChange={(next) => !action.isPending && setRejectOpen(next)}
        title="Tolak sesi opname?"
        description="Sesi kembali ke pengaju untuk diperbaiki. Alasan wajib diisi."
        footer={
          <>
            <Button variant="secondary" onClick={() => setRejectOpen(false)} disabled={action.isPending}>
              Kembali
            </Button>
            <Button
              variant="danger"
              loading={action.isPending}
              disabled={rejectReason.trim() === ''}
              onClick={() => action.mutate('reject')}
            >
              Tolak sesi
            </Button>
          </>
        }
      >
        <Input
          label="Alasan penolakan"
          value={rejectReason}
          onChange={(event) => setRejectReason(event.target.value)}
        />
      </Dialog>
    </>
  )
}

function OpnameRow({
  item,
  editable,
  value,
  onChange,
}: {
  item: OpnameItem
  editable: boolean
  value: { quantity: string; reason: string }
  onChange: (value: { quantity: string; reason: string }) => void
}) {
  const counted = item.counted_quantity !== undefined && item.counted_quantity !== null
  const difference = item.difference_quantity ? Number(item.difference_quantity) : 0

  return (
    <li className="grid gap-3 px-5 py-3 md:grid-cols-[minmax(0,1fr)_10rem_10rem] md:items-center">
      <div className="min-w-0">
        <p className="text-sm font-medium">{item.ingredient_name}</p>
        <p className="num text-xs text-muted">
          {item.ingredient_code} · sistem {quantity(item.system_quantity)} {item.base_unit_code}
        </p>
        {counted && !editable && (
          <p className="text-xs text-muted">
            Dihitung {quantity(item.counted_quantity ?? '0')} {item.base_unit_code}
            {item.difference_value ? ` · nilai selisih ${formatMoney(item.difference_value)}` : ''}
          </p>
        )}
      </div>

      {editable ? (
        <>
          <Input
            aria-label={`Hasil hitung ${item.ingredient_name}`}
            inputMode="decimal"
            placeholder="Belum dihitung"
            value={value.quantity}
            onChange={(event) => onChange({ ...value, quantity: event.target.value })}
            trailing={<span className="text-xs text-muted">{item.base_unit_code}</span>}
          />
          <Input
            aria-label={`Alasan selisih ${item.ingredient_name}`}
            placeholder="Alasan selisih"
            value={value.reason}
            onChange={(event) => onChange({ ...value, reason: event.target.value })}
          />
        </>
      ) : (
        <>
          <span className="num text-sm">
            {counted ? quantity(item.counted_quantity ?? '0') : '-'} {item.base_unit_code}
          </span>
          <span
            className={cn(
              'num text-sm',
              difference === 0 ? 'text-muted' : difference > 0 ? 'text-warning' : 'text-negative',
            )}
          >
            {counted ? `${difference > 0 ? '+' : ''}${quantity(String(difference))}` : '-'}
          </span>
        </>
      )}
    </li>
  )
}

const ACTION_MESSAGES = {
  start: 'Sesi opname dimulai.',
  submit: 'Sesi diajukan untuk persetujuan.',
  approve: 'Sesi disetujui dan stok disesuaikan.',
  cancel: 'Sesi dibatalkan.',
  reject: 'Sesi ditolak.',
} as const

function quantity(value: string): string {
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return value
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 3 }).format(parsed)
}
