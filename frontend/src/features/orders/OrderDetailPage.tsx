import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Printer, XCircle } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { SERVICE_TYPE_LABELS, orderApi, paymentMethodLabel } from '../../api/orders'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { printReceipt } from '../../utils/printReceipt'
import { orderStatusLabel, orderStatusTone, paymentStatusLabel, paymentStatusTone } from '../../utils/status'

export function OrderDetailPage() {
  const { id = '' } = useParams()
  const toast = useToast()
  const queryClient = useQueryClient()
  const [reason, setReason] = useState('')
  const [cancelOpen, setCancelOpen] = useState(false)

  const order = useQuery({
    queryKey: ['order', id],
    queryFn: async () => (await orderApi.get(id)).data.data,
    enabled: id !== '',
    refetchInterval: 15_000,
  })
  const payments = useQuery({
    queryKey: ['order-payments', id],
    queryFn: async () => (await orderApi.payments(id)).data.data,
    enabled: id !== '',
  })

  const print = useMutation({
    mutationFn: async () => (await orderApi.receipt(id)).data.data,
    onSuccess: (receipt) => printReceipt(receipt),
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const cancel = useMutation({
    mutationFn: () =>
      orderApi.cancel(id, {
        reason_code: 'CANCELLED_BY_CASHIER',
        note: reason.trim() || null,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['order', id] })
      await queryClient.invalidateQueries({ queryKey: ['orders'] })
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      setCancelOpen(false)
      setReason('')
      toast.push('Pesanan dibatalkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  if (order.isLoading) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner label="Memuat pesanan" />
      </div>
    )
  }
  if (order.isError || !order.data) {
    return (
      <Card className="p-6 text-sm text-negative">
        {apiMessage(order.error ?? new Error('Pesanan tidak ditemukan.'))}
      </Card>
    )
  }

  const data = order.data
  const items = data.items ?? []
  const cancellable = data.status !== 'CANCELLED' && data.status !== 'COMPLETED'

  return (
    <>
      <Link
        to="/orders"
        className="inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua pesanan
      </Link>

      <div className="mt-2">
        <PageHeader
          title={data.order_no ?? 'Draft pesanan'}
          description={`${SERVICE_TYPE_LABELS[data.service_type]} · ${formatDate(data.created_at)}`}
          action={
            <>
              <Button variant="secondary" loading={print.isPending} onClick={() => print.mutate()}>
                <Printer className="size-4" strokeWidth={1.5} />
                Cetak struk
              </Button>
              {cancellable && (
                <Button variant="danger" onClick={() => setCancelOpen(true)}>
                  <XCircle className="size-4" strokeWidth={1.5} />
                  Batalkan
                </Button>
              )}
            </>
          }
        />
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="space-y-4">
          <Card>
            <CardHeader
              title="Item pesanan"
              action={
                <div className="flex flex-wrap gap-1">
                  <Badge tone={orderStatusTone(data.status)}>{orderStatusLabel(data.status)}</Badge>
                  <Badge tone={paymentStatusTone(data.payment_status)}>
                    {paymentStatusLabel(data.payment_status)}
                  </Badge>
                  {data.priority === 'RUSH' && <Badge tone="yellow">RUSH</Badge>}
                </div>
              }
            />
            {items.length === 0 ? (
              <CardBody className="text-sm text-muted">Pesanan belum memiliki item.</CardBody>
            ) : (
              <ul className="divide-y divide-rule">
                {items.map((item) => (
                  <li
                    key={item.id}
                    className={cn('flex justify-between gap-3 px-5 py-3', item.status === 'VOID' && 'opacity-50')}
                  >
                    <div className="min-w-0">
                      <p className={cn('text-sm font-medium', item.status === 'VOID' && 'line-through')}>
                        {item.product_name}
                        {item.status === 'VOID' && <span className="ml-2 text-xs text-negative">DIBATALKAN</span>}
                      </p>
                      <p className="text-xs text-muted">
                        {item.variant_name} · {item.quantity} x {formatMoney(item.unit_price)}
                      </p>
                      {item.modifiers && item.modifiers.length > 0 && (
                        <p className="text-xs text-muted">
                          {item.modifiers.map((modifier) => modifier.option_name).join(', ')}
                        </p>
                      )}
                      {item.note && <p className="text-xs text-muted">Catatan: {item.note}</p>}
                    </div>
                    <span className="num shrink-0 text-sm">{formatMoney(item.line_total)}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card>
            <CardHeader title="Pembayaran" />
            <CardBody className="p-0">
              {payments.isLoading ? (
                <div className="p-5">
                  <Spinner label="Memuat pembayaran" />
                </div>
              ) : (payments.data ?? []).length === 0 ? (
                <p className="p-5 text-sm text-muted">Belum ada pembayaran.</p>
              ) : (
                <ul className="divide-y divide-rule">
                  {(payments.data ?? []).map((payment) => (
                    <li key={payment.id} className="flex items-center justify-between gap-3 px-5 py-3">
                      <div>
                        <p className="text-sm font-medium">{paymentMethodLabel(payment.method_code)}</p>
                        <p className="text-xs text-muted">
                          {payment.paid_at ? formatDate(payment.paid_at) : 'Menunggu'}
                          {payment.reference_no ? ` · ${payment.reference_no}` : ''}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="num text-sm">{formatMoney(payment.amount)}</p>
                        <Badge tone={payment.status === 'PAID' || payment.status === 'SUCCESS' ? 'green' : 'gray'}>
                          {payment.status === 'SUCCESS' ? 'Berhasil' : payment.status}
                        </Badge>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </CardBody>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader title="Ringkasan" />
            <CardBody className="space-y-2">
              <SummaryRow label="Subtotal" value={data.subtotal_amount} />
              <SummaryRow label="Diskon item" value={data.item_discount_amount} />
              <SummaryRow label="Diskon pesanan" value={data.order_discount_amount} />
              <SummaryRow label="Service" value={data.service_charge_amount} />
              <SummaryRow label="Pajak" value={data.tax_amount} />
              <SummaryRow label="Pembulatan" value={data.rounding_amount} />
              <div className="flex items-center justify-between border-t border-rule pt-2 font-semibold">
                <span>Total</span>
                <span className="num">{formatMoney(data.total_amount)}</span>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted">Dibayar</span>
                <span className="num">{formatMoney(data.paid_amount)}</span>
              </div>
            </CardBody>
          </Card>

          {(data.customer_name || data.customer_phone || data.delivery_address) && (
            <Card>
              <CardHeader title="Pelanggan" />
              <CardBody className="space-y-1 text-sm">
                {data.customer_name && <p className="font-medium">{data.customer_name}</p>}
                {data.customer_phone && <p className="text-muted">{data.customer_phone}</p>}
                {data.delivery_address && <p className="text-muted">{data.delivery_address}</p>}
              </CardBody>
            </Card>
          )}
        </div>
      </div>

      <Dialog
        open={cancelOpen}
        onOpenChange={(open) => !cancel.isPending && setCancelOpen(open)}
        title="Batalkan pesanan?"
        description="Stok dikembalikan bila belum dimasak. Setelah masuk dapur, pembatalan dicatat sebagai waste dan butuh izin manager."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCancelOpen(false)} disabled={cancel.isPending}>
              Kembali
            </Button>
            <Button variant="danger" loading={cancel.isPending} onClick={() => cancel.mutate()}>
              Batalkan pesanan
            </Button>
          </>
        }
      >
        <label className="block text-sm font-medium text-ink" htmlFor="cancel-reason">
          Alasan
        </label>
        <textarea
          id="cancel-reason"
          rows={3}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder="Contoh: pelanggan membatalkan pesanan"
          className="mt-2 w-full rounded-control border border-control bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/20"
        />
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
