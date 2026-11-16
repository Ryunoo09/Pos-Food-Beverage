import { useQuery } from '@tanstack/react-query'
import { reportApi } from '../../api/reports'
import type { MenuPerformance, MenuPerformanceReport } from '../../api/reports'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'
import { useReportPeriod } from './period'
import { ReportShell } from './ReportShell'

const percent = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 })

export function MenuPerformanceReportPage() {
  const { from, to, setPeriod } = useReportPeriod()

  const report = useQuery({
    queryKey: ['report-menu', { from, to }],
    queryFn: async () => (await reportApi.menuPerformance({ from, to })).data.data,
  })

  const data = report.data
  const items = data?.items ?? []

  const columns: Column<MenuPerformance>[] = [
    {
      key: 'name',
      header: 'Menu',
      render: (row) => (
        <div>
          <p className="text-sm">{row.name}</p>
          {row.variant_name && <p className="text-xs text-faint">{row.variant_name}</p>}
        </div>
      ),
    },
    {
      key: 'qty',
      header: 'Terjual',
      className: 'text-right',
      render: (row) => <span className="num tabular-nums">{row.quantity}</span>,
    },
    {
      key: 'revenue',
      header: 'Omzet',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.revenue)}</span>,
    },
    {
      key: 'cost',
      header: 'HPP',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.total_cost)}</span>,
    },
    {
      key: 'profit',
      header: 'Laba kotor',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span
          className={
            Number(row.gross_profit) < 0 ? 'num tabular-nums text-negative' : 'num tabular-nums'
          }
        >
          {formatMoney(row.gross_profit)}
        </span>
      ),
    },
    {
      key: 'margin',
      header: 'Margin',
      className: 'text-right whitespace-nowrap',
      render: (row) => {
        // Margin null berarti HPP item tidak diketahui; menampilkan 0% akan
        // terbaca seperti menu tanpa laba, jadi dinyatakan apa adanya.
        if (row.margin_percent === null) return <Badge tone="gray">HPP belum ada</Badge>
        const margin = Number(row.margin_percent)
        return (
          <Badge tone={marginTone(margin)}>
            {percent.format(margin)}%
          </Badge>
        )
      },
    },
  ]

  return (
    <>
      <PageHeader
        title="Performa menu"
        description="Omzet, HPP tersimpan, dan margin tiap menu pada periode terpilih. Menu tanpa HPP tidak dihitung marginnya."
      />

      <ReportShell kind="menu-performance" from={from} to={to} onPeriodChange={setPeriod}>
        {report.isError ? (
          <Card>
            <div className="px-5 py-8 text-center">
              <p className="text-sm text-negative">{apiMessage(report.error)}</p>
            </div>
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4">
            <Tiles data={data} loading={report.isLoading} />

            <Card>
              <CardHeader title="Omzet dan margin per menu" />
              <DataTable
                columns={columns}
                rows={items}
                rowKey={(row) => `${row.product_id}-${row.variant_name}`}
                loading={report.isLoading}
                empty="Belum ada penjualan menu pada periode ini."
              />
            </Card>
          </div>
        )}
      </ReportShell>
    </>
  )
}

function Tiles({ data, loading }: { data?: MenuPerformanceReport; loading: boolean }) {
  const items = data?.items ?? []
  const known = items.filter((item) => item.margin_percent !== null)
  // Margin terendah dibaca sebagai teks agar tidak ada aritmetika uang.
  const weakest = known.reduce<MenuPerformance | undefined>(
    (low, item) =>
      low === undefined || Number(item.margin_percent) < Number(low.margin_percent) ? item : low,
    undefined,
  )

  const tiles: { label: string; value?: string; hint?: string }[] = [
    { label: 'Menu terjual', value: data ? String(items.length) : undefined, hint: 'varian' },
    { label: 'Menu punya HPP', value: data ? String(known.length) : undefined, hint: 'dihitung marginnya' },
    {
      label: 'Margin terendah',
      // Dengan satu menu, "margin terendah" sama dengan margin satu-satunya
      // dan tidak memberi informasi; tampilkan bahwa sampelnya terlalu kecil.
      value: known.length < 2 ? '—' : `${percent.format(Number(weakest?.margin_percent ?? 0))}%`,
      hint: known.length < 2 ? 'perlu minimal 2 menu' : weakest?.name,
    },
    {
      label: 'Menu tanpa HPP',
      value: data ? String(items.length - known.length) : undefined,
      hint: 'margin tidak dihitung',
    },
  ]

  return (
    <Card>
      <div className="grid grid-cols-2 divide-line sm:grid-cols-4">
        {tiles.map((tile) => (
          <div key={tile.label} className="px-4 py-3">
            <p className="text-xs text-muted">{tile.label}</p>
            {loading || tile.value === undefined ? (
              <Skeleton className="mt-2 h-6 w-20" />
            ) : (
              <p className="num mt-1 text-[15px] leading-tight tabular-nums">{tile.value}</p>
            )}
            {tile.hint && <p className="truncate text-[11px] text-faint">{tile.hint}</p>}
          </div>
        ))}
      </div>
    </Card>
  )
}

function marginTone(margin: number): 'green' | 'yellow' | 'red' {
  if (margin < 0) return 'red'
  if (margin < 30) return 'yellow'
  return 'green'
}
