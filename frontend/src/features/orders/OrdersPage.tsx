import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ORDER_STATUSES, SERVICE_TYPE_LABELS, orderApi } from '../../api/orders'
import type { Order } from '../../api/orders'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Card } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { orderStatusLabel, orderStatusTone, paymentStatusLabel, paymentStatusTone } from '../../utils/status'

const perPage = 20

export function OrdersPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const status = params.get('status') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [search, setSearch] = useState(q)

  const orders = useQuery({
    queryKey: ['orders', { q, status, page }],
    queryFn: async () =>
      (
        await orderApi.list({
          q: q || undefined,
          status: status || undefined,
          page,
          per_page: perPage,
        })
      ).data,
    refetchInterval: 15_000,
  })

  function patchParams(patch: Record<string, string>) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    if (!('page' in patch)) next.delete('page')
    setParams(next, { replace: true })
  }

  const columns: Column<Order>[] = [
    {
      key: 'order',
      header: 'Pesanan',
      render: (row) => (
        <div>
          <p className="num font-medium text-ink">{row.order_no ?? 'Draft'}</p>
          <p className="text-xs text-muted">
            {row.queue_no ? `Antrean ${row.queue_no} · ` : ''}
            {SERVICE_TYPE_LABELS[row.service_type]}
          </p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <div className="flex flex-wrap gap-1">
          <Badge tone={orderStatusTone(row.status)}>{orderStatusLabel(row.status)}</Badge>
          <Badge tone={paymentStatusTone(row.payment_status)}>
            {paymentStatusLabel(row.payment_status)}
          </Badge>
          {row.priority === 'RUSH' && <Badge tone="yellow">RUSH</Badge>}
        </div>
      ),
    },
    {
      key: 'total',
      header: 'Total',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <div className="num">
          <p>{formatMoney(row.total_amount)}</p>
          <p className="text-xs text-muted">Dibayar {formatMoney(row.paid_amount)}</p>
        </div>
      ),
    },
    {
      key: 'created',
      header: 'Waktu',
      className: 'whitespace-nowrap text-sm text-muted',
      render: (row) => formatDate(row.created_at),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) => (
        <Link
          to={`/orders/${row.id}`}
          className="inline-flex min-h-11 items-center rounded-control px-3 text-sm font-medium text-ink hover:bg-raised"
        >
          Detail
        </Link>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Pesanan"
        description="Semua pesanan hari ini beserta status dapur dan pembayarannya."
      />

      <div className="mt-6 flex flex-wrap items-end gap-3">
        <form
          className="min-w-[16rem] flex-1"
          onSubmit={(event) => {
            event.preventDefault()
            patchParams({ q: search.trim() })
          }}
        >
          <Input
            aria-label="Cari pesanan"
            placeholder="Cari nomor pesanan atau antrean…"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            trailing={
              <button type="submit" aria-label="Cari" className="grid size-11 place-items-center text-muted hover:text-ink">
                <Search className="size-4" strokeWidth={1.5} />
              </button>
            }
          />
        </form>
        <div className="w-full sm:w-52">
          <Select
            ariaLabel="Filter status"
            value={status || 'all'}
            onValueChange={(value) => patchParams({ status: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua status' },
              ...ORDER_STATUSES.map((item) => ({ value: item, label: item })),
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={orders.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={orders.isLoading}
          error={orders.isError ? apiMessage(orders.error) : undefined}
          onRetry={() => orders.refetch()}
          empty="Belum ada pesanan yang cocok."
        />
        <Pagination
          page={orders.data?.meta.page ?? page}
          totalPages={orders.data?.meta.total_pages ?? 1}
          total={orders.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>
    </>
  )
}
