import { useQuery } from '@tanstack/react-query'
import { reportApi } from '../../api/reports'
import type { ShiftLine, ShiftReport } from '../../api/reports'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'
import { shiftStatusLabel, shiftStatusTone } from '../../utils/status'
import { useReportPeriod } from './period'
import { ReportShell } from './ReportShell'

const METHOD_LABELS: Record<string, string> = {
  CASH: 'Tunai',
  QRIS: 'QRIS',
  TRANSFER: 'Transfer',
  E_WALLET: 'E-wallet',
}

const methodLabel = (code: string) => METHOD_LABELS[code] ?? code

export function ShiftReportPage() {
  const { from, to, setPeriod } = useReportPeriod()

  const report = useQuery({
    queryKey: ['report-shifts', { from, to }],
    queryFn: async () => (await reportApi.shifts({ from, to })).data.data,
  })

  const data = report.data
  const shifts = data?.shifts ?? []

  const columns: Column<ShiftLine>[] = [
    {
      key: 'cashier',
      header: 'Kasir',
      render: (row) => (
        <div>
          <p className="text-sm">{row.cashier_name}</p>
          <p className="num text-xs text-faint">{timeRange(row.opened_at, row.closed_at)}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={shiftStatusTone(row.status)}>{shiftStatusLabel(row.status)}</Badge>
      ),
    },
    {
      key: 'opening',
      header: 'Kas awal',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.opening_cash)}</span>,
    },
    {
      key: 'sales',
      header: 'Penjualan',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.sales)}</span>,
    },
    {
      key: 'expected',
      header: 'Kas seharusnya',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{optional(row.expected_cash)}</span>,
    },
    {
      key: 'actual',
      header: 'Kas aktual',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{optional(row.actual_cash)}</span>,
    },
    {
      key: 'difference',
      header: 'Selisih',
      className: 'text-right whitespace-nowrap',
      render: (row) => <Difference value={row.difference} />,
    },
    {
      key: 'methods',
      header: 'Metode bayar',
      render: (row) =>
        row.payment_methods.length === 0 ? (
          <span className="text-xs text-faint">Tidak ada pembayaran</span>
        ) : (
          <ul className="space-y-0.5">
            {row.payment_methods.map((method) => (
              <li key={method.method} className="flex items-baseline gap-2 text-xs">
                <span className="text-muted">{methodLabel(method.method)}</span>
                <span className="num tabular-nums">{formatMoney(method.amount)}</span>
              </li>
            ))}
          </ul>
        ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Laporan shift"
        description="Rekap kas per shift kasir pada periode terpilih, termasuk selisih kas dan komposisi metode bayar."
      />

      <ReportShell kind="shifts" from={from} to={to} onPeriodChange={setPeriod}>
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
              <CardHeader title="Shift pada periode" />
              <DataTable
                columns={columns}
                rows={shifts}
                rowKey={(row) => row.id}
                loading={report.isLoading}
                empty="Belum ada shift pada periode ini."
              />
            </Card>
          </div>
        )}
      </ReportShell>
    </>
  )
}

/**
 * Selisih kas ditampilkan dengan tanda plus/minus dan warna; shift yang belum
 * ditutup tidak punya selisih, jadi ditulis "—" bukan nol.
 */
function Difference({ value }: { value?: string }) {
  if (value === undefined) return <span className="text-faint">—</span>

  const amount = Number(value)
  if (amount === 0) {
    return <span className="num tabular-nums text-muted">{formatMoney(value)}</span>
  }
  return (
    <span
      className={`num tabular-nums ${amount < 0 ? 'text-negative' : 'text-warning'}`}
    >
      {amount > 0 ? '+' : ''}
      {formatMoney(value)}
    </span>
  )
}

function Tiles({ data, loading }: { data?: ShiftReport; loading: boolean }) {
  const shifts = data?.shifts ?? []
  const open = shifts.filter((shift) => shift.status === 'OPEN').length
  // Selisih dijumlah di sini hanya untuk indikator "berapa shift berselisih",
  // bukan nominal uang yang ditampilkan.
  const mismatched = shifts.filter(
    (shift) => shift.difference !== undefined && Number(shift.difference) !== 0,
  ).length

  const tiles: { label: string; value?: string; hint?: string }[] = [
    { label: 'Total shift', value: data ? String(shifts.length) : undefined },
    { label: 'Masih terbuka', value: data ? String(open) : undefined, hint: 'belum ditutup' },
    { label: 'Kas berselisih', value: data ? String(mismatched) : undefined, hint: 'shift' },
    {
      label: 'Ditinjau',
      value: data ? String(shifts.filter((shift) => shift.status === 'REVIEWED').length) : undefined,
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
            {tile.hint && <p className="text-[11px] text-faint">{tile.hint}</p>}
          </div>
        ))}
      </div>
    </Card>
  )
}

/** Nilai kas yang belum ada (shift terbuka) ditulis "—", bukan Rp 0. */
function optional(value?: string): string {
  return value === undefined ? '—' : formatMoney(value)
}

/** "2026-11-14 · 14:03 – 22:11" dari dua timestamp ISO. */
function timeRange(openedAt: string, closedAt?: string): string {
  const start = openedAt.slice(11, 16)
  const end = closedAt ? closedAt.slice(11, 16) : 'berjalan'
  return `${openedAt.slice(0, 10)} · ${start} – ${end}`
}
