import { useQuery } from '@tanstack/react-query'
import { reportApi } from '../../api/reports'
import type { InventoryReport, MovementLine, StockLine } from '../../api/reports'
import { wasteReasonLabel } from '../../api/waste'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'
import { opnameStatusLabel, opnameStatusTone } from '../../utils/status'
import { useReportPeriod } from './period'
import { ReportShell } from './ReportShell'

const qty = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 })

const MOVEMENT_LABELS: Record<string, string> = {
  INITIAL_STOCK: 'Stok awal',
  PURCHASE: 'Pembelian',
  SALE: 'Terjual',
  SALE_REVERSAL: 'Pembatalan penjualan',
  WASTE: 'Waste',
  WASTE_REVERSAL: 'Pembatalan waste',
  ADJUSTMENT: 'Penyesuaian opname',
  RETURN: 'Retur',
  TRANSFER_OUT: 'Transfer keluar',
  TRANSFER_IN: 'Transfer masuk',
}

const movementLabel = (type: string) => MOVEMENT_LABELS[type] ?? type

const STOCK_STATUS: Record<string, { label: string; tone: 'green' | 'yellow' | 'red' }> = {
  OK: { label: 'Aman', tone: 'green' },
  LOW: { label: 'Menipis', tone: 'yellow' },
  CRITICAL: { label: 'Kritis', tone: 'red' },
}

const reasonLabel = (code: string) =>
  code === 'TANPA_ALASAN' ? 'Tanpa alasan' : wasteReasonLabel(code)

export function InventoryReportPage() {
  const { from, to, setPeriod } = useReportPeriod()

  const report = useQuery({
    queryKey: ['report-inventory', { from, to }],
    queryFn: async () => (await reportApi.inventory({ from, to })).data.data,
  })

  const data = report.data
  // Bahan yang perlu perhatian diberi urutan sendiri: kritis lalu menipis,
  // karena tekanan stok adalah hal yang dicari pertama saat membuka laporan.
  const stock = [...(data?.stock ?? [])].sort(
    (a, b) => stockRank(a.status) - stockRank(b.status) || a.name.localeCompare(b.name),
  )
  const attention = (data?.stock ?? []).filter((item) => item.status !== 'OK').length

  const stockColumns: Column<StockLine>[] = [
    {
      key: 'name',
      header: 'Bahan',
      render: (row) => (
        <div>
          <p className="text-sm">{row.name}</p>
          <p className="num text-xs text-faint">{row.code}</p>
        </div>
      ),
    },
    {
      key: 'onhand',
      header: 'Stok',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num tabular-nums">
          {qty.format(Number(row.quantity_on_hand))} {row.unit}
        </span>
      ),
    },
    {
      key: 'min',
      header: 'Minimum',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num text-muted tabular-nums">
          {qty.format(Number(row.min_stock))} {row.unit}
        </span>
      ),
    },
    {
      key: 'cost',
      header: 'HPP rata-rata',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num tabular-nums">{formatMoney(row.avg_unit_cost)}</span>
      ),
    },
    {
      key: 'value',
      header: 'Nilai stok',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.value)}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => {
        const status = STOCK_STATUS[row.status] ?? { label: row.status, tone: 'green' as const }
        return <Badge tone={status.tone}>{status.label}</Badge>
      },
    },
  ]

  const movementColumns: Column<MovementLine>[] = [
    { key: 'name', header: 'Bahan', render: (row) => row.name },
    { key: 'type', header: 'Jenis mutasi', render: (row) => movementLabel(row.movement_type) },
    {
      key: 'qty',
      header: 'Perubahan',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        // Tanda plus/minus menjaga arah mutasi terbaca tanpa mengandalkan warna.
        <span className="num tabular-nums">
          {Number(row.quantity) > 0 ? '+' : ''}
          {qty.format(Number(row.quantity))}
        </span>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Laporan inventaris"
        description="Stok dan valuasi saat ini, ringkasan mutasi, waste, dan hasil stock opname pada periode terpilih."
      />

      <ReportShell kind="inventory" from={from} to={to} onPeriodChange={setPeriod}>
        {report.isError ? (
          <ErrorCard message={apiMessage(report.error)} />
        ) : (
          <div className="grid grid-cols-1 gap-4">
            <Tiles data={data} attention={attention} loading={report.isLoading} />

            <Card>
              <CardHeader
                title={
                  <span className="flex flex-wrap items-baseline gap-x-2">
                    Stok saat ini
                    <span className="text-xs font-normal text-muted">
                      posisi hari ini, bukan akhir periode
                    </span>
                  </span>
                }
              />
              <DataTable
                columns={stockColumns}
                rows={stock}
                rowKey={(row) => row.ingredient_id}
                loading={report.isLoading}
                empty="Belum ada bahan aktif."
              />
            </Card>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <Card>
                <CardHeader title="Ringkasan mutasi stok" />
                <DataTable
                  columns={movementColumns}
                  rows={data?.movements ?? []}
                  rowKey={(row) => `${row.ingredient_id}-${row.movement_type}`}
                  loading={report.isLoading}
                  empty="Belum ada mutasi stok pada periode ini."
                />
              </Card>

              <Card>
                <CardHeader title="Waste per alasan" />
                <DataTable
                  columns={[
                    { key: 'reason', header: 'Alasan', render: (row) => reasonLabel(row.reason_code) },
                    {
                      key: 'count',
                      header: 'Catatan',
                      className: 'text-right',
                      render: (row) => row.count,
                    },
                    {
                      key: 'cost',
                      header: 'Nilai',
                      className: 'text-right whitespace-nowrap',
                      render: (row) => (
                        <span className="num tabular-nums">{formatMoney(row.cost_value)}</span>
                      ),
                    },
                  ]}
                  rows={data?.waste ?? []}
                  rowKey={(row) => row.reason_code}
                  loading={report.isLoading}
                  empty="Tidak ada waste pada periode ini."
                />
              </Card>
            </div>

            <Card>
              <CardHeader title="Hasil stock opname" />
              <DataTable
                columns={[
                  {
                    key: 'code',
                    header: 'Nomor',
                    render: (row) => <span className="num">{row.opname_no}</span>,
                  },
                  {
                    key: 'status',
                    header: 'Status',
                    render: (row) => (
                      <Badge tone={opnameStatusTone(row.status)}>
                        {opnameStatusLabel(row.status)}
                      </Badge>
                    ),
                  },
                  {
                    key: 'date',
                    header: 'Diselesaikan',
                    render: (row) => <span className="num">{shortDate(row.finished_at)}</span>,
                  },
                  {
                    key: 'qty',
                    header: 'Selisih kuantitas',
                    className: 'text-right whitespace-nowrap',
                    render: (row) => (
                      <span className="num tabular-nums">
                        {qty.format(Number(row.total_difference_quantity))}
                      </span>
                    ),
                  },
                  {
                    key: 'value',
                    header: 'Selisih nilai',
                    className: 'text-right whitespace-nowrap',
                    render: (row) => (
                      <span className="num tabular-nums">
                        {formatMoney(row.total_difference_value)}
                      </span>
                    ),
                  },
                ]}
                rows={data?.opnames ?? []}
                rowKey={(row) => row.id}
                loading={report.isLoading}
                empty="Belum ada opname selesai pada periode ini."
              />
            </Card>
          </div>
        )}
      </ReportShell>
    </>
  )
}

function Tiles({
  data,
  attention,
  loading,
}: {
  data?: InventoryReport
  attention: number
  loading: boolean
}) {
  const tiles = [
    { label: 'Bahan aktif', value: data ? String(data.stock.length) : undefined, money: false },
    {
      label: 'Perlu perhatian',
      value: data ? String(attention) : undefined,
      money: false,
      hint: 'stok menipis atau kritis',
    },
    { label: 'Nilai stok', value: data?.stock_value, money: true },
    { label: 'Total waste', value: data?.waste_value, money: true },
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
              <p className="num mt-1 text-[15px] leading-tight tabular-nums">
                {tile.money ? formatMoney(tile.value) : tile.value}
              </p>
            )}
            {tile.hint && <p className="text-[11px] text-faint">{tile.hint}</p>}
          </div>
        ))}
      </div>
    </Card>
  )
}

function ErrorCard({ message }: { message: string }) {
  return (
    <Card>
      <div className="px-5 py-8 text-center">
        <p className="text-sm text-negative">{message}</p>
      </div>
    </Card>
  )
}

function stockRank(status: string): number {
  switch (status) {
    case 'CRITICAL':
      return 0
    case 'LOW':
      return 1
    default:
      return 2
  }
}

/** Waktu ISO dari server dipangkas ke tanggal agar kolom tetap rapat. */
function shortDate(iso: string): string {
  return iso ? iso.slice(0, 10) : '-'
}
