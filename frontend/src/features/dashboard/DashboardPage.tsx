import { ArrowRight } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { PageHeader } from '../../components/shared/PageHeader'
import { formatMoney } from '../../utils/formatMoney'
import { cn } from '../../utils/cn'
import { LineChart } from './LineChart'
import {
  averageOrder,
  lastWeek,
  lowStock,
  recentOrders,
  today,
  totalOrders,
  totalSales,
} from './dashboardData'

type SeriesKey = 'sales' | 'orders' | 'average'

const percent = new Intl.NumberFormat('id-ID', {
  style: 'percent',
  maximumFractionDigits: 1,
  signDisplay: 'exceptZero',
})

const quantity = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 })

function shortRupiah(value: number): string {
  if (value === 0) return '0'
  if (value >= 1_000_000) return `${quantity.format(value / 1_000_000)}jt`
  return `${quantity.format(value / 1_000)}rb`
}

interface SeriesConfig {
  name: string
  ticks: number[]
  formatTick: (value: number) => string
  formatValue: (value: number) => string
  values: { label: string; value: number }[]
}

const seriesConfig: Record<SeriesKey, SeriesConfig> = {
  sales: {
    name: 'Penjualan per jam',
    ticks: [0, 250_000, 500_000, 750_000, 1_000_000],
    formatTick: shortRupiah,
    formatValue: (value: number) => formatMoney(String(Math.round(value))),
    values: today.hours.map((bucket) => ({ label: `${bucket.hour}.00`, value: bucket.sales })),
  },
  orders: {
    name: 'Pesanan per jam',
    ticks: [0, 5, 10, 15],
    formatTick: (value: number) => String(value),
    formatValue: (value: number) => `${value} pesanan`,
    values: today.hours.map((bucket) => ({ label: `${bucket.hour}.00`, value: bucket.orders })),
  },
  average: {
    name: 'Rata-rata per pesanan',
    ticks: [0, 20_000, 40_000, 60_000, 80_000],
    formatTick: shortRupiah,
    formatValue: (value: number) => formatMoney(String(Math.round(value))),
    values: today.hours.map((bucket) => ({
      label: `${bucket.hour}.00`,
      value: bucket.sales / bucket.orders,
    })),
  },
}

const orderTone = {
  Dikonfirmasi: 'gray',
  Dimasak: 'blue',
  Siap: 'yellow',
  Selesai: 'green',
} as const

export function DashboardPage() {
  const [series, setSeries] = useState<SeriesKey>('sales')
  const active = seriesConfig[series]

  const salesDelta = (totalSales - lastWeek.sales) / lastWeek.sales
  const ordersDelta = (totalOrders - lastWeek.orders) / lastWeek.orders
  const lastWeekAverage = lastWeek.sales / lastWeek.orders
  const averageDelta = (averageOrder - lastWeekAverage) / lastWeekAverage

  const peakHour = today.hours.reduce((best, bucket) => (bucket.sales > best.sales ? bucket : best))

  const tabs = [
    {
      key: 'sales' as const,
      label: 'Penjualan',
      value: formatMoney(String(totalSales)),
      delta: salesDelta,
    },
    {
      key: 'orders' as const,
      label: 'Pesanan',
      value: `${totalOrders}`,
      delta: ordersDelta,
    },
    {
      key: 'average' as const,
      label: 'Rata-rata per pesanan',
      value: formatMoney(String(averageOrder)),
      delta: averageDelta,
    },
  ]

  return (
    <>
      <PageHeader
        title="Ringkasan"
        description={`${today.date} · puncak jam ${peakHour.hour}.00 dengan ${peakHour.orders} pesanan`}
        action={
          <Link to="/pos">
            <Button>Buka kasir</Button>
          </Link>
        }
      />

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-12">
        <Card className="lg:col-span-12">
          <div className="grid grid-cols-1 border-b border-line sm:grid-cols-3">
            {tabs.map((tab) => {
              const selected = tab.key === series
              return (
                <button
                  key={tab.key}
                  type="button"
                  onClick={() => setSeries(tab.key)}
                  aria-pressed={selected}
                  className={cn(
                    'flex items-center justify-between gap-3 border-t border-line px-5 py-3 text-left transition-colors first:border-t-0 sm:block sm:border-t-0 sm:border-l sm:px-5 sm:py-4 sm:first:border-l-0',
                    'border-b-2',
                    selected
                      ? 'border-b-ink bg-raised sm:bg-transparent'
                      : 'border-b-transparent hover:bg-raised',
                  )}
                >
                  <span className="min-w-0">
                    <span className={cn('block text-xs', selected ? 'text-ink' : 'text-muted')}>
                      {tab.label}
                    </span>
                    <span className="mt-0.5 block text-xs text-muted sm:hidden">
                      vs Minggu lalu{' '}
                      <span className={tab.delta > 0 ? 'text-positive' : 'text-muted'}>
                        {percent.format(tab.delta)}
                      </span>
                    </span>
                  </span>
                  <span className="num shrink-0 text-[20px] leading-tight text-ink sm:mt-1 sm:block sm:text-[26px]">
                    {tab.value}
                  </span>
                  <span className="hidden text-xs text-muted sm:block">
                    vs Minggu lalu{' '}
                    <span className={tab.delta > 0 ? 'text-positive' : 'text-muted'}>
                      {percent.format(tab.delta)}
                    </span>
                  </span>
                </button>
              )
            })}
          </div>

          <div className="px-5 pt-5 pb-3">
            <LineChart
              seriesName={active.name}
              points={active.values}
              ticks={active.ticks}
              formatTick={active.formatTick}
              formatValue={active.formatValue}
              annotation={(point) => `${point.label} · ${active.formatValue(point.value)}`}
            />
          </div>
        </Card>

        <Card className="lg:col-span-7">
          <CardHeader
            title="Pesanan terakhir"
            action={
              <Link
                to="/orders"
                className="inline-flex items-center gap-1 text-[13px] text-muted hover:text-ink"
              >
                Semua pesanan
                <ArrowRight className="size-3.5" strokeWidth={1.5} />
              </Link>
            }
          />
          <ul className="divide-y divide-rule">
            {recentOrders.map((order) => (
              <li key={order.code} className="flex min-h-12 items-center gap-3 px-5 py-2">
                <div className="min-w-0 flex-1">
                  <p className="num text-[13px] leading-tight">{order.code}</p>
                  <p className="text-xs leading-tight text-muted">
                    {order.place} · {order.items} item
                  </p>
                </div>
                <span className="num text-[13px] whitespace-nowrap">
                  {formatMoney(String(order.total))}
                </span>
                <span className="w-20 text-right">
                  <Badge tone={orderTone[order.status]}>{order.status}</Badge>
                </span>
              </li>
            ))}
          </ul>
        </Card>

        <Card className="lg:col-span-5">
          <CardHeader
            title="Stok menipis"
            action={
              <Link
                to="/inventory"
                className="inline-flex items-center gap-1 text-[13px] text-muted hover:text-ink"
              >
                Semua bahan
                <ArrowRight className="size-3.5" strokeWidth={1.5} />
              </Link>
            }
          />
          <ul className="divide-y divide-rule">
            {lowStock.map((item) => {
              const critical = item.left / item.minimum <= 0.5
              return (
                <li key={item.name} className="flex min-h-12 items-center gap-3 px-5 py-2">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[13px] leading-tight">{item.name}</p>
                    <p className="text-xs leading-tight text-muted">
                      minimum {quantity.format(item.minimum)} {item.unit}
                    </p>
                  </div>
                  <span
                    className={cn(
                      'num text-[13px] whitespace-nowrap',
                      critical ? 'text-negative' : 'text-ink',
                    )}
                  >
                    {quantity.format(item.left)} {item.unit}
                  </span>
                </li>
              )
            })}
          </ul>
        </Card>
      </div>
    </>
  )
}
