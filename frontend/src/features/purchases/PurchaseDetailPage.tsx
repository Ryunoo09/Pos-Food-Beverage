import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, PackageCheck, Send, ShieldCheck, XCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { PO_STATUS_LABELS, purchaseApi } from '../../api/purchases'
import type { PurchaseOrderItem, Receiving } from '../../api/purchases'
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
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { generateIdempotencyKey } from '../../utils/idempotency'
import { purchaseStatusTone } from '../../utils/status'

export function PurchaseDetailPage() {
  const { id = '' } = useParams()
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)

  const [cancelOpen, setCancelOpen] = useState(false)
  const [cancelReason, setCancelReason] = useState('')
  const [receiveOpen, setReceiveOpen] = useState(false)
  const [receiveKey, setReceiveKey] = useState('')
  const [invoiceNo, setInvoiceNo] = useState('')
  const [receiveNote, setReceiveNote] = useState('')
  const [quantities, setQuantities] = useState<Record<string, string>>({})
  const [receivedSummary, setReceivedSummary] = useState<Receiving | null>(null)

  const po = useQuery({
    queryKey: ['purchase-order', id],
    queryFn: async () => (await purchaseApi.get(id)).data.data,
    enabled: id !== '',
  })

  function invalidate() {
    return Promise.all([
      queryClient.invalidateQueries({ queryKey: ['purchase-order', id] }),
      queryClient.invalidateQueries({ queryKey: ['purchase-orders'] }),
      queryClient.invalidateQueries({ queryKey: ['stock'] }),
    ])
  }

  const transition = useMutation({
    mutationFn: async (kind: 'submit' | 'approve' | 'cancel') => {
      if (kind === 'submit') return purchaseApi.submit(id)
      if (kind === 'approve') return purchaseApi.approve(id)
      return purchaseApi.cancel(id, cancelReason.trim())
    },
    onSuccess: async (_result, kind) => {
      if (kind === 'cancel') {
        setCancelOpen(false)
        setCancelReason('')
      }
      await invalidate()
      toast.push(
        kind === 'submit'
          ? 'PO diajukan untuk persetujuan.'
          : kind === 'approve'
            ? 'PO disetujui.'
            : 'PO dibatalkan.',
      )
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const receive = useMutation({
    mutationFn: async () => {
      if (!po.data) throw new Error('PO belum dimuat.')
      const items = (po.data.items ?? [])
        .filter((item) => Number(quantities[item.id] ?? '0') > 0)
        .map((item) => ({
          po_item_id: item.id,
          ingredient_id: item.ingredient_id,
          purchase_unit_id: item.purchase_unit_id,
          quantity_received: (quantities[item.id] ?? '').trim(),
          unit_price: item.unit_price,
          discount_allocated: '0',
        }))
      return purchaseApi.receive(id, receiveKey, {
        supplier_id: po.data.supplier_id,
        supplier_invoice_no: invoiceNo.trim() || null,
        note: receiveNote.trim() || null,
        items,
      })
    },
    onSuccess: async (result) => {
      setReceiveOpen(false)
      setQuantities({})
      setInvoiceNo('')
      setReceiveNote('')
      setReceivedSummary(result.data.data)
      await invalidate()
      toast.push('Penerimaan dicatat dan stok diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const outstanding = useMemo(() => {
    const items = po.data?.items ?? []
    return items.reduce(
      (total, item) =>
        total + Math.max(0, Number(item.base_quantity_ordered) - Number(item.quantity_received_base)),
      0,
    )
  }, [po.data?.items])

  if (po.isLoading) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner label="Memuat purchase order" />
      </div>
    )
  }
  if (po.isError || !po.data) {
    return (
      <Card className="p-6 text-sm text-negative">
        {apiMessage(po.error ?? new Error('Purchase order tidak ditemukan.'))}
      </Card>
    )
  }

  const data = po.data
  const items = data.items ?? []
  const canSubmit = data.status === 'DRAFT' && permissions.includes('purchases.submit')
  const canApprove = data.status === 'SUBMITTED' && permissions.includes('purchases.approve')
  const canCancel =
    data.status !== 'RECEIVED' &&
    data.status !== 'CANCELLED' &&
    permissions.includes('purchases.cancel')
  const canReceive =
    (data.status === 'APPROVED' || data.status === 'PARTIALLY_RECEIVED') &&
    permissions.includes('purchases.receive')

  return (
    <>
      <Link
        to="/purchase"
        className="inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua pembelian
      </Link>

      <div className="mt-2">
        <PageHeader
          title={data.code}
          description={`${data.supplier_name} · dibuat ${formatDate(data.created_at)}`}
          action={
            <>
              {canSubmit && (
                <Button loading={transition.isPending} onClick={() => transition.mutate('submit')}>
                  <Send className="size-4" strokeWidth={1.5} />
                  Ajukan
                </Button>
              )}
              {canApprove && (
                <Button loading={transition.isPending} onClick={() => transition.mutate('approve')}>
                  <ShieldCheck className="size-4" strokeWidth={1.5} />
                  Setujui
                </Button>
              )}
              {canReceive && (
                <Button
                  onClick={() => {
                    setReceiveKey(generateIdempotencyKey())
                    setReceiveOpen(true)
                  }}
                >
                  <PackageCheck className="size-4" strokeWidth={1.5} />
                  Terima barang
                </Button>
              )}
              {canCancel && (
                <Button
                  variant="secondary"
                  onClick={() => setCancelOpen(true)}
                  disabled={transition.isPending}
                >
                  <XCircle className="size-4" strokeWidth={1.5} />
                  Batalkan
                </Button>
              )}
            </>
          }
        />
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-muted">
        <Badge tone={purchaseStatusTone(data.status)}>
          {PO_STATUS_LABELS[data.status] ?? data.status}
        </Badge>
        {data.expected_date && <span>Perkiraan datang {data.expected_date}</span>}
        {(data.status === 'APPROVED' || data.status === 'PARTIALLY_RECEIVED') && (
          <span>· sisa {formatQty(outstanding)} unit dasar belum diterima</span>
        )}
      </div>

      {receivedSummary && (
        <p className="mt-3 rounded-control bg-pill-positive px-3 py-2 text-sm text-pill-positive-ink">
          Penerimaan {receivedSummary.code} tercatat: {receivedSummary.items?.length ?? 0} baris.
        </p>
      )}

      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <Card>
          <CardHeader title="Item pesanan" />
          {items.length === 0 ? (
            <CardBody className="text-sm text-muted">PO ini belum punya item.</CardBody>
          ) : (
            <ul className="divide-y divide-rule">
              {items.map((item) => (
                <PurchaseItemRow key={item.id} item={item} />
              ))}
            </ul>
          )}
        </Card>

        <Card>
          <CardHeader title="Ringkasan" />
          <CardBody className="space-y-2">
            <Row label="Subtotal" value={data.subtotal} />
            <Row label="Diskon" value={data.discount_amount} />
            <div className="flex items-center justify-between border-t border-rule pt-2 font-semibold">
              <span>Total</span>
              <span className="num">{formatMoney(data.total_amount)}</span>
            </div>
            {data.note && <p className="pt-2 text-xs text-muted">Catatan: {data.note}</p>}
          </CardBody>
        </Card>
      </div>

      <Dialog
        open={receiveOpen}
        onOpenChange={(next) => !receive.isPending && setReceiveOpen(next)}
        title={`Terima barang ${data.code}`}
        description="Isi jumlah yang benar-benar diterima. Baris kosong dianggap belum datang."
        footer={
          <>
            <Button variant="secondary" onClick={() => setReceiveOpen(false)} disabled={receive.isPending}>
              Batal
            </Button>
            <Button
              loading={receive.isPending}
              disabled={
                Object.values(quantities).every((value) => Number(value || '0') <= 0)
              }
              onClick={() => receive.mutate()}
            >
              Posting penerimaan
            </Button>
          </>
        }
      >
        <div className="max-h-[65dvh] space-y-3 overflow-y-auto pr-1">
          <Input
            label="Nomor invoice supplier"
            value={invoiceNo}
            onChange={(event) => setInvoiceNo(event.target.value)}
          />
          <Input label="Catatan" value={receiveNote} onChange={(event) => setReceiveNote(event.target.value)} />
          <ul className="divide-y divide-rule border-y border-rule">
            {items.map((item) => {
              const remaining = Math.max(
                0,
                Number(item.base_quantity_ordered) - Number(item.quantity_received_base),
              )
              if (remaining === 0) return null
              return (
                <li key={item.id} className="py-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{item.ingredient_name}</p>
                      <p className="text-xs text-muted">
                        Sisa dalam satuan beli {item.purchase_unit_code}
                      </p>
                    </div>
                    <Input
                      aria-label={`Jumlah diterima ${item.ingredient_name}`}
                      inputMode="decimal"
                      placeholder="0"
                      value={quantities[item.id] ?? ''}
                      onChange={(event) =>
                        setQuantities((current) => ({ ...current, [item.id]: event.target.value }))
                      }
                      trailing={<span className="text-xs text-muted">{item.purchase_unit_code}</span>}
                    />
                  </div>
                </li>
              )
            })}
          </ul>
        </div>
      </Dialog>

      <Dialog
        open={cancelOpen}
        onOpenChange={(next) => !transition.isPending && setCancelOpen(next)}
        title="Batalkan purchase order?"
        description="PO yang dibatalkan tidak bisa diterima lagi."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCancelOpen(false)} disabled={transition.isPending}>
              Kembali
            </Button>
            <Button
              variant="danger"
              loading={transition.isPending}
              disabled={cancelReason.trim() === ''}
              onClick={() => transition.mutate('cancel')}
            >
              Batalkan PO
            </Button>
          </>
        }
      >
        <Input
          label="Alasan"
          value={cancelReason}
          onChange={(event) => setCancelReason(event.target.value)}
        />
      </Dialog>
    </>
  )
}

function PurchaseItemRow({ item }: { item: PurchaseOrderItem }) {
  const ordered = Number(item.base_quantity_ordered)
  const received = Number(item.quantity_received_base)
  const progress = ordered > 0 ? Math.min(100, Math.round((received / ordered) * 100)) : 0

  return (
    <li className="px-5 py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium">{item.ingredient_name}</p>
          <p className="num text-xs text-muted">
            {item.quantity_ordered} {item.purchase_unit_code} × {formatMoney(item.unit_price)}
          </p>
        </div>
        <span className="num shrink-0 text-sm">{formatMoney(item.line_total)}</span>
      </div>
      <div className="mt-2">
        <div className="h-1.5 w-full overflow-hidden rounded-chip bg-raised">
          <div className="h-full rounded-chip bg-brand-600" style={{ width: `${progress}%` }} />
        </div>
        <p className="mt-1 text-xs text-muted">
          Diterima {formatQty(received)} dari {formatQty(ordered)} {item.purchase_unit_code}
        </p>
      </div>
    </li>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted">{label}</span>
      <span className="num">{formatMoney(value)}</span>
    </div>
  )
}

function formatQty(value: number): string {
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 3 }).format(value)
}
