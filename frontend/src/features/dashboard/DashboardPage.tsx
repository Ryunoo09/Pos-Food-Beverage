import { useQuery } from '@tanstack/react-query'
import { ArrowRight, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { dashboardApi, PAYMENT_METHOD_LABELS } from '../../api/dashboard'
import type { DashboardSummary } from '../../api/dashboard'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Skeleton } from '../../components/ui/Skeleton'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatMoney } from '../../utils/formatMoney'
import { orderStatusLabel, orderStatusTone } from '../../utils/status'
import { LineChart } from './LineChart'

type SeriesKey = 'sales' | 'orders' | 'average'

const quantity = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 })

/** Label pendek untuk sumbu chart; uang tetap dibaca dari string server. */
function shortRupiah(value: number): string {
  if (value === 0) return '0'
  if (value >= 1_000_000) return `${quantity.format(value / 1_000_000)}jt`
  return `${quantity.format(value / 1_000)}rb`
}

function todayISO(): string {
  const now = new Date()
  const offset = now.getTimezoneOffset() * 60_000
  return new Date(now.getTime() - offset).toISOString().slice(0, 10)
}

export function DashboardPage() {
  const permissions = useAuthStore((state) => state.permissions)
  const canSeeProfit = permissions.includes('reports.profit.view')

  const [params, setParams] = useSearchParams()
  const from = params.get('from') || todayISO()
  const to = params.get('to') || from

  const [series, setSeries] = useState<SeriesKey>('sales')

  const summary = useQuery({
    queryKey: ['dashboard', { from, to }],
    queryFn: async () => (await dashboardApi.summary({ from, to })).data.data,
    // Widget tidak perlu realtime; segarkan tiap menit seperti spesifikasi.
    refetchInterval: 60_000,
  })

  function patchRange(patch: { from?: string; to?: string }) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    setParams(next, { replace: true })
  }

  function presetRange(days: number) {
    const end = new Date()
    const start = new Date(end.getTime() - (days - 1) * 86_400_000)
    const iso = (date: Date) =>
      new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
    setParams(new URLSearchParams({ from: iso(start), to: iso(end) }), { replace: true })
  }

  const data = summary.data

  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Ringkasan penjualan dan operasional outlet pada periode terpilih."
        action={
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => summary.refetch()}
              disabled={summary.isFetching}
              aria-label="Muat ulang data"
            >
              <RefreshCw
                className={cn('size-4', summary.isFetching && 'animate-spin')}
                strokeWidth={1.5}
              />
              Muat ulang
            </Button>
          </div>
        }
      />

      <PeriodBar
        from={from}
        to={to}
        onChange={patchRange}
        onPreset={presetRange}
        fetching={summary.isFetching}
      />

      {summary.isError ? (
        <Card className="mt-4">
          <div className="px-5 py-8 text-center">
            <p className="text-sm text-negative">{apiMessage(summary.error)}</p>
            <Button variant="secondary" size="sm" className="mt-3" onClick={() => summary.refetch()}>
              Coba lagi
            </Button>
          </div>
        </Card>
      ) : (
        <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-12">
          <SummaryStrip
            data={data}
            loading={summary.isLoading}
            canSeeProfit={canSeeProfit}
            series={series}
            onSeriesChange={setSeries}
          />

          <Card className="lg:col-span-8">
            <CardHeader
              title={series === 'sales' ? 'Penjualan per jam' : series === 'orders' ? 'Pesanan per jam' : 'Rata-rata per pesanan'}
              action={
                <span className="num text-xs text-muted">
                  {from === to ? from : `${from} – ${to}`}
                </span>
              }
            />
            <div className="px-5 pt-5 pb-3">
              <HourlyChart data={data} series={series} loading={summary.isLoading} />
            </div>
          </Card>

          <Card className="lg:col-span-4">
            <CardHeader title="Stok kritis" />
            <CriticalList rows={data?.critical_stock ?? []} loading={summary.isLoading} />
          </Card>

          <TopProducts rows={data?.top_products ?? []} loading={summary.isLoading} />

          <PaymentMethods rows={data?.payment_methods ?? []} loading={summary.isLoading} />

          <ActiveOrders rows={data?.active_orders ?? []} loading={summary.isLoading} />
        </div>
      )}
    </>
  )
}

function PeriodBar({
  from,
  to,
  onChange,
  onPreset,
  fetching,
}: {
  from: string
  to: string
  onChange: (patch: { from?: string; to?: string }) => void
  onPreset: (days: number) => void
  fetching: boolean
}) {
  const inputClass =
    'h-11 rounded-control border border-control bg-surface px-3 text-sm text-ink outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/20'

  return (
    <div className="mt-4 flex flex-wrap items-center gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="date"
          aria-label="Dari tanggal"
          className={inputClass}
          value={from}
          max={to || undefined}
          onChange={(event) => onChange({ from: event.target.value })}
        />
        <span className="text-faint">s/d</span>
        <input
          type="date"
          aria-label="Sampai tanggal"
          className={inputClass}
          value={to}
          min={from || undefined}
          onChange={(event) => onChange({ to: event.target.value })}
        />
      </div>

      <div className="flex items-center gap-1">
        <Button variant="ghost" size="sm" onClick={() => onPreset(1)}>
          Hari ini
        </Button>
        <Button variant="ghost" size="sm" onClick={() => onPreset(7)}>
          7 hari
        </Button>
        <Button variant="ghost" size="sm" onClick={() => onPreset(30)}>
          30 hari
        </Button>
      </div>

      {fetching && <span className="text-xs text-muted">Memuat…</span>}
    </div>
  )
}

function SummaryStrip({
  data,
  loading,
  canSeeProfit,
  series,
  onSeriesChange,
}: {
  data?: DashboardSummary
  loading: boolean
  canSeeProfit: boolean
  series: SeriesKey
  onSeriesChange: (key: SeriesKey) => void
}) {
  const tabs: { key: SeriesKey; label: string; value: string | undefined; money: boolean }[] = [
    { key: 'sales', label: 'Penjualan bersih', value: data?.net_sales, money: true },
    {
      key: 'orders',
      label: 'Transaksi',
      value: data ? String(data.transactions) : undefined,
      money: false,
    },
    { key: 'average', label: 'Rata-rata pesanan', value: data?.aov, money: true },
  ]

  const profitTabs = canSeeProfit
    ? [
        { key: 'hpp' as const, label: 'HPP', value: data?.hpp, money: true },
        { key: 'profit' as const, label: 'Laba kotor', value: data?.gross_profit, money: true },
      ]
    : []

  return (
    <Card className="lg:col-span-12">
      <div className={cn('grid grid-cols-1', canSeeProfit ? 'sm:grid-cols-5' : 'sm:grid-cols-3')}>
        {tabs.map((tab) => {
          const selected = tab.key === series
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => onSeriesChange(tab.key)}
              aria-pressed={selected}
              className={cn(
                // Pemilih seri: garis brand di kaki kolom menandai pilihan tanpa
                // menambah blok latar yang menabrak hairline kartu.
                'group relative flex items-center justify-between gap-3 px-5 py-3 text-left transition-colors sm:block sm:py-4',
                'border-t border-line first:border-t-0 sm:border-t-0 sm:border-l sm:first:border-l-0',
                selected ? 'bg-raised/60 sm:bg-transparent' : 'hover:bg-raised/40',
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  'absolute inset-x-0 bottom-0 h-0.5 origin-left bg-brand-600 transition-transform duration-200',
                  selected ? 'scale-x-100' : 'scale-x-0',
                )}
              />
              <span
                className={cn(
                  'block text-xs transition-colors',
                  selected ? 'font-medium text-ink' : 'text-muted group-hover:text-ink',
                )}
              >
                {tab.label}
              </span>
              {loading ? (
                <Skeleton className="mt-2 h-7 w-24" />
              ) : (
                <span className="num shrink-0 text-[20px] leading-tight text-ink tabular-nums sm:mt-1.5 sm:block sm:text-[24px]">
                  {tab.value === undefined ? '—' : tab.money ? formatMoney(tab.value) : tab.value}
                </span>
              )}
            </button>
          )
        })}

        {profitTabs.map((tab) => (
          <div
            key={tab.key}
            className="flex items-center justify-between gap-3 border-t border-line px-5 py-3 sm:block sm:border-t-0 sm:border-l sm:py-4"
          >
            <span className="block text-xs text-muted">{tab.label}</span>
            {loading ? (
              <Skeleton className="mt-2 h-7 w-24" />
            ) : (
              <span className="num shrink-0 text-[20px] leading-tight text-ink tabular-nums sm:mt-1.5 sm:block sm:text-[24px]">
                {tab.value === undefined ? '—' : formatMoney(tab.value)}
              </span>
            )}
          </div>
        ))}
      </div>
    </Card>
  )
}

function HourlyChart({
  data,
  series,
  loading,
}: {
  data?: DashboardSummary
  series: SeriesKey
  loading: boolean
}) {
  if (loading) {
    return <Skeleton className="h-[180px] w-full" />
  }

  const hourly = data?.hourly_sales ?? []
  if (hourly.length === 0) {
    return <p className="py-12 text-center text-sm text-muted">Belum ada data pada periode ini.</p>
  }

  const totalSales = Number(data?.net_sales ?? '0')

  // Nilai chart diturunkan langsung dari angka server per jam; tidak ada
  // aritmetika uang yang dipakai untuk menampilkan total.
  const points = hourly.map((bucket) => {
    if (series === 'orders') {
      return { label: `${bucket.hour}.00`, value: bucket.transactions }
    }
    if (series === 'average') {
      const sales = Number(bucket.sales)
      return {
        label: `${bucket.hour}.00`,
        value: bucket.transactions > 0 ? sales / bucket.transactions : 0,
      }
    }
    return { label: `${bucket.hour}.00`, value: Number(bucket.sales) }
  })

  const peakValue = points.reduce((max, point) => Math.max(max, point.value), 0)
  const ticks = axisTicks(peakValue, series === 'orders')

  const formatTick = series === 'orders' ? (value: number) => String(Math.round(value)) : shortRupiah
  const formatValue =
    series === 'orders'
      ? (value: number) => `${Math.round(value)} pesanan`
      : (value: number) => formatMoney(String(Math.round(value)))

  return (
    <>
      <LineChart
        seriesName={series === 'sales' ? 'Penjualan per jam' : series === 'orders' ? 'Pesanan per jam' : 'Rata-rata per pesanan'}
        points={points}
        ticks={ticks}
        formatTick={formatTick}
        formatValue={formatValue}
        annotation={(point) => `${point.label} · ${formatValue(point.value)}`}
      />
      <p className="mt-2 text-xs text-muted">
        Total penjualan periode: <span className="num">{formatMoney(String(totalSales))}</span>
      </p>
    </>
  )
}

/**
 * Gridline sumbu. Untuk nilai cacah (jumlah pesanan) jarak antar-tick harus
 * bilangan bulat, kalau tidak labelnya membulat dan angkanya berulang ("2, 2").
 */
function axisTicks(peak: number, discrete: boolean): number[] {
  if (discrete) {
    const top = Math.max(4, Math.ceil(peak))
    // Naikkan batas atas bila hasilnya tidak habis dibagi 4 agar tick bulat.
    const step = Math.ceil(top / 4)
    const max = step * 4
    return [0, step, step * 2, step * 3, max]
  }
  if (peak <= 0) return [0, 25_000, 50_000, 75_000, 100_000]
  const magnitude = 10 ** Math.floor(Math.log10(peak))
  const step = magnitude / 2
  const max = Math.ceil(peak / step) * step
  return [0, max / 4, max / 2, (max * 3) / 4, max]
}

function CriticalList({
  rows,
  loading,
}: {
  rows: DashboardSummary['critical_stock']
  loading: boolean
}) {
  if (loading) {
    return (
      <div className="space-y-3 p-5">
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className="h-10 w-full" />
        ))}
      </div>
    )
  }

  if (rows.length === 0) {
    return <p className="px-5 py-10 text-center text-sm text-muted">Semua stok di atas minimum.</p>
  }

  return (
    <ul className="divide-y divide-rule">
      {rows.map((item) => {
        const onHand = Number(item.quantity_on_hand)
        const min = Number(item.min_stock)
        const ratio = min > 0 ? onHand / min : 1
        const critical = ratio <= 0.5
        return (
          <li key={item.ingredient_id} className="flex min-h-12 items-center gap-3 px-5 py-2.5">
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] leading-tight">{item.name}</p>
              <div className="mt-1.5 flex items-center gap-2">
                <span
                  className="h-1 w-16 shrink-0 overflow-hidden rounded-full bg-rule"
                  aria-hidden="true"
                >
                  <span
                    className={cn('block h-full rounded-full', critical ? 'bg-negative' : 'bg-warning')}
                    style={{ width: `${Math.min(Math.max(ratio, 0.04), 1) * 100}%` }}
                  />
                </span>
                <p className="truncate text-xs leading-tight text-muted">
                  min {quantity.format(min)}
                </p>
              </div>
            </div>
            <span
              className={cn(
                'num text-[13px] whitespace-nowrap tabular-nums',
                critical ? 'font-medium text-negative' : 'text-ink',
              )}
            >
              {quantity.format(onHand)}
            </span>
          </li>
        )
      })}
    </ul>
  )
}

function TopProducts({
  rows,
  loading,
}: {
  rows: DashboardSummary['top_products']
  loading: boolean
}) {
  return (
    <Card className="lg:col-span-7">
      <CardHeader title="Menu terlaris" />
      {loading ? (
        <div className="space-y-3 p-5">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-8 w-full" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <p className="px-5 py-10 text-center text-sm text-muted">Belum ada penjualan.</p>
      ) : (
        <ul className="divide-y divide-rule">
          {rows.map((item, index) => (
            <li key={item.product_id} className="flex min-h-12 items-center gap-3 px-5 py-2">
              <span className="num w-5 shrink-0 text-xs text-faint tabular-nums">{index + 1}</span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[13px] leading-tight">{item.name}</p>
                <p className="text-xs leading-tight text-muted">{item.quantity} terjual</p>
              </div>
              <span className="num text-[13px] whitespace-nowrap tabular-nums">
                {formatMoney(item.sales)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

function PaymentMethods({
  rows,
  loading,
}: {
  rows: DashboardSummary['payment_methods']
  loading: boolean
}) {
  const total = rows.reduce((sum, row) => sum + Number(row.amount), 0)

  return (
    <Card className="lg:col-span-5">
      <CardHeader title="Metode bayar" />
      {loading ? (
        <div className="space-y-3 p-5">
          {Array.from({ length: 3 }, (_, index) => (
            <Skeleton key={index} className="h-8 w-full" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <p className="px-5 py-10 text-center text-sm text-muted">Belum ada pembayaran.</p>
      ) : (
        <ul className="divide-y divide-rule">
          {rows.map((row) => {
            const share = total > 0 ? Number(row.amount) / total : 0
            return (
              <li key={row.method} className="flex items-center gap-3 px-5 py-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline justify-between gap-2">
                    <p className="truncate text-[13px]">
                      {PAYMENT_METHOD_LABELS[row.method] ?? row.method}
                    </p>
                    <span className="num text-[13px] whitespace-nowrap tabular-nums">
                      {formatMoney(row.amount)}
                    </span>
                  </div>
                  <span
                    className="mt-1.5 block h-1 w-full overflow-hidden rounded-full bg-rule"
                    aria-hidden="true"
                  >
                    <span
                      className="block h-full rounded-full bg-chart-1"
                      style={{ width: `${Math.min(Math.max(share, 0), 1) * 100}%` }}
                    />
                  </span>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </Card>
  )
}

function ActiveOrders({
  rows,
  loading,
}: {
  rows: DashboardSummary['active_orders']
  loading: boolean
}) {
  return (
    <Card className="lg:col-span-12">
      <CardHeader
        title="Pesanan aktif"
        action={
          <Link
            to="/orders"
            className="inline-flex min-h-9 items-center gap-1 text-xs font-medium text-ink underline underline-offset-4"
          >
            Lihat semua
            <ArrowRight className="size-3.5" strokeWidth={2} />
          </Link>
        }
      />
      {loading ? (
        <div className="p-5">
          <Skeleton className="h-8 w-full" />
        </div>
      ) : rows.length === 0 ? (
        <p className="px-5 py-8 text-center text-sm text-muted">Tidak ada pesanan aktif.</p>
      ) : (
        <ul className="flex flex-wrap gap-x-6 gap-y-3 px-5 py-4">
          {rows.map((row) => (
            <li key={row.status} className="flex items-center gap-2">
              <Badge tone={orderStatusTone(row.status)}>{orderStatusLabel(row.status)}</Badge>
              <span className="num text-sm tabular-nums">{row.count}</span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
