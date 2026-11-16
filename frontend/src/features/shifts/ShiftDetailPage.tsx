import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowDownLeft, ArrowUpRight, Lock } from 'lucide-react'
import { useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { shiftApi } from '../../api/shifts'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { generateIdempotencyKey } from '../../utils/idempotency'

export function ShiftDetailPage() {
  const { id = '' } = useParams()
  const toast = useToast()
  const queryClient = useQueryClient()
  const closeKey = useRef<string | null>(null)
  const [closeOpen, setCloseOpen] = useState(false)
  const [actualCash, setActualCash] = useState('')
  const [differenceReason, setDifferenceReason] = useState('')
  const [movementOpen, setMovementOpen] = useState(false)
  const [movementType, setMovementType] = useState<'IN' | 'OUT'>('IN')
  const [movementAmount, setMovementAmount] = useState('')
  const [movementCategory, setMovementCategory] = useState('')
  const [movementReason, setMovementReason] = useState('')

  const summary = useQuery({
    queryKey: ['shift', id],
    queryFn: async () => (await shiftApi.summary(id)).data.data,
    enabled: id !== '',
  })
  const movements = useQuery({
    queryKey: ['shift-movements', id],
    queryFn: async () => (await shiftApi.cashMovements(id)).data.data,
    enabled: id !== '',
  })

  const close = useMutation({
    mutationFn: () => {
      closeKey.current ??= generateIdempotencyKey()
      return shiftApi.close(id, closeKey.current, {
        actual_cash: actualCash.trim(),
        difference_reason: differenceReason.trim() || null,
      })
    },
    onSuccess: async () => {
      closeKey.current = null
      setCloseOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['shift', id] }),
        queryClient.invalidateQueries({ queryKey: ['shifts'] }),
        queryClient.invalidateQueries({ queryKey: ['shift-current'] }),
      ])
      toast.push('Shift ditutup.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const movement = useMutation({
    mutationFn: () =>
      shiftApi.addCashMovement(id, {
        type: movementType,
        amount: movementAmount.trim(),
        category: movementCategory.trim() || null,
        reason: movementReason.trim(),
      }),
    onSuccess: async () => {
      setMovementOpen(false)
      setMovementAmount('')
      setMovementCategory('')
      setMovementReason('')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['shift', id] }),
        queryClient.invalidateQueries({ queryKey: ['shift-movements', id] }),
      ])
      toast.push('Kas dicatat.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  if (summary.isLoading) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner label="Memuat shift" />
      </div>
    )
  }
  if (summary.isError || !summary.data) {
    return (
      <Card className="p-6 text-sm text-negative">
        {apiMessage(summary.error ?? new Error('Shift tidak ditemukan.'))}
      </Card>
    )
  }

  const data = summary.data
  const shift = data.shift
  const isOpen = shift.status === 'OPEN'

  return (
    <>
      <Link
        to="/shifts"
        className="inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua shift
      </Link>

      <div className="mt-2">
        <PageHeader
          title={`Shift ${formatDate(shift.opened_at)}`}
          description={
            shift.closed_at
              ? `Ditutup ${formatDate(shift.closed_at)}`
              : 'Shift masih berjalan. Tutup untuk merekonsiliasi kas.'
          }
          action={
            isOpen && (
              <>
                <Button variant="secondary" onClick={() => setMovementOpen(true)}>
                  Catat kas
                </Button>
                <Button onClick={() => setCloseOpen(true)}>
                  <Lock className="size-4" strokeWidth={1.5} />
                  Tutup shift
                </Button>
              </>
            )
          }
        />
      </div>

      <div className="mt-3">
        <Badge tone={isOpen ? 'green' : 'gray'}>{isOpen ? 'Terbuka' : 'Ditutup'}</Badge>
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <Card>
          <CardHeader title="Kas masuk & keluar" />
          <CardBody className="p-0">
            {movements.isLoading ? (
              <div className="p-5">
                <Spinner label="Memuat kas" />
              </div>
            ) : (movements.data ?? []).length === 0 ? (
              <p className="p-5 text-sm text-muted">Belum ada kas masuk atau keluar.</p>
            ) : (
              <ul className="divide-y divide-rule">
                {(movements.data ?? []).map((row) => (
                  <li key={row.id} className="flex items-center justify-between gap-3 px-5 py-3">
                    <div className="flex min-w-0 items-start gap-2">
                      {row.type === 'IN' ? (
                        <ArrowDownLeft className="mt-0.5 size-4 shrink-0 text-positive" strokeWidth={1.5} />
                      ) : (
                        <ArrowUpRight className="mt-0.5 size-4 shrink-0 text-negative" strokeWidth={1.5} />
                      )}
                      <div className="min-w-0">
                        <p className="text-sm font-medium">{row.reason ?? row.category ?? 'Kas'}</p>
                        <p className="text-xs text-muted">
                          {formatDate(row.created_at)}
                          {row.category ? ` · ${row.category}` : ''}
                        </p>
                      </div>
                    </div>
                    <span className="num shrink-0 text-sm">
                      {row.type === 'IN' ? '+' : '-'}
                      {formatMoney(row.amount)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </CardBody>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader title="Ringkasan kas" />
            <CardBody className="space-y-2">
              <SummaryRow label="Modal awal" value={shift.opening_cash} />
              <SummaryRow label="Penjualan tunai" value={data.cash_sales} />
              <SummaryRow label="Refund tunai" value={data.cash_refunds} />
              <SummaryRow label="Kas masuk" value={data.cash_in} />
              <SummaryRow label="Kas keluar" value={data.cash_out} />
              <div className="flex items-center justify-between border-t border-rule pt-2 font-semibold">
                <span>Kas seharusnya</span>
                <span className="num">{formatMoney(data.expected_cash)}</span>
              </div>
              {shift.actual_cash && <SummaryRow label="Kas aktual" value={shift.actual_cash} />}
              {shift.difference && <SummaryRow label="Selisih" value={shift.difference} />}
              {shift.difference_reason && (
                <p className="text-xs text-muted">Alasan: {shift.difference_reason}</p>
              )}
            </CardBody>
          </Card>

          <Card>
            <CardHeader title="Per metode bayar" />
            <CardBody className="space-y-2">
              {data.by_method.length === 0 ? (
                <p className="text-sm text-muted">Belum ada pembayaran.</p>
              ) : (
                data.by_method.map((method) => (
                  <div key={method.method_code} className="flex items-center justify-between text-sm">
                    <span className="text-muted">
                      {method.method_code} · {method.count}x
                    </span>
                    <span className="num">{formatMoney(method.amount)}</span>
                  </div>
                ))
              )}
              <div className="border-t border-rule pt-2 text-xs text-muted">
                <p>Pembayaran void: {data.voided_payment_count} · {formatMoney(data.voided_payment_amount)}</p>
                <p>Pembayaran menggantung: {data.pending_payment_count}</p>
              </div>
            </CardBody>
          </Card>
        </div>
      </div>

      <Dialog
        open={movementOpen}
        onOpenChange={(next) => !movement.isPending && setMovementOpen(next)}
        title="Catat kas masuk / keluar"
        description="Catatan ini memengaruhi kas seharusnya pada rekonsiliasi shift."
        footer={
          <>
            <Button variant="secondary" onClick={() => setMovementOpen(false)} disabled={movement.isPending}>
              Batal
            </Button>
            <Button
              loading={movement.isPending}
              onClick={() => movement.mutate()}
              disabled={
                !/^\d+(\.\d+)?$/.test(movementAmount.trim()) ||
                Number(movementAmount) <= 0 ||
                movementReason.trim() === ''
              }
            >
              Simpan
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Select
            label="Jenis"
            value={movementType}
            onValueChange={(value) => setMovementType(value as 'IN' | 'OUT')}
            options={[
              { value: 'IN', label: 'Kas masuk' },
              { value: 'OUT', label: 'Kas keluar' },
            ]}
          />
          <Input
            label="Jumlah"
            inputMode="decimal"
            value={movementAmount}
            onChange={(event) => setMovementAmount(event.target.value)}
          />
          <Input
            label="Kategori"
            placeholder="Contoh: setoran, pembelian mendadak"
            value={movementCategory}
            onChange={(event) => setMovementCategory(event.target.value)}
          />
          <Input
            label="Alasan"
            value={movementReason}
            onChange={(event) => setMovementReason(event.target.value)}
          />
        </div>
      </Dialog>

      <Dialog
        open={closeOpen}
        onOpenChange={(next) => !close.isPending && setCloseOpen(next)}
        title="Tutup shift"
        description={`Kas seharusnya ${formatMoney(data.expected_cash)}. Isi kas aktual hasil hitung fisik.`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setCloseOpen(false)} disabled={close.isPending}>
              Batal
            </Button>
            <Button
              variant="danger"
              loading={close.isPending}
              onClick={() => close.mutate()}
              disabled={!/^\d+(\.\d+)?$/.test(actualCash.trim())}
            >
              Tutup shift
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Kas aktual"
            inputMode="decimal"
            value={actualCash}
            onChange={(event) => setActualCash(event.target.value)}
          />
          <Input
            label="Alasan selisih"
            placeholder="Wajib bila kas aktual tidak sama dengan kas seharusnya"
            value={differenceReason}
            onChange={(event) => setDifferenceReason(event.target.value)}
          />
        </div>
      </Dialog>
    </>
  )
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted">{label}</span>
      <span className="num">{formatMoney(value)}</span>
    </div>
  )
}
