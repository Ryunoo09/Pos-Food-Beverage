import { useQuery } from '@tanstack/react-query'
import { reportApi } from '../../api/reports'
import type { SalesBucket, SalesReport } from '../../api/reports'
import { PageHeader } from '../../components/shared/PageHeader'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'
import { useReportPeriod } from './period'
import { ReportShell } from './ReportShell'

const percent = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 })

export function SalesReportPage() {
  const { from, to, setPeriod } = useReportPeriod()

  const report = useQuery({
    queryKey: ['report-sales', { from, to }],
    queryFn: async () => (await reportApi.sales({ from, to })).data.data,
  })

  const data = report.data
  // Hari tanpa transaksi disembunyikan dari tabel supaya periode panjang
  // tidak menjadi ratusan baris nol; total tetap mencakup seluruh periode.
  const activeDays = (data?.daily ?? []).filter((day) => day.order_count > 0)

  const dayColumns: Column<SalesBucket>[] = [
    { key: 'date', header: 'Tanggal', render: (row) => <span className="num">{row.key}</span> },
    { key: 'orders', header: 'Pesanan', className: 'text-right', render: (row) => row.order_count },
    {
      key: 'gross',
      header: 'Penjualan kotor',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.gross_sales)}</span>,
    },
    {
      key: 'net',
      header: 'Penjualan bersih',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.net_sales)}</span>,
    },
    {
      key: 'total',
      header: 'Total ditagih',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.total)}</span>,
    },
  ]

  const bucketColumns = (label: string): Column<SalesBucket>[] => [
    { key: 'label', header: label, render: (row) => row.label },
    { key: 'orders', header: 'Pesanan', className: 'text-right', render: (row) => row.order_count },
    {
      key: 'net',
      header: 'Penjualan bersih',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num tabular-nums">{formatMoney(row.net_sales)}</span>,
    },
    {
      key: 'share',
      header: 'Porsi',
      className: 'text-right whitespace-nowrap',
      render: (row) => {
        const total = Number(data?.total.net_sales ?? '0')
        // Porsi hanya untuk membaca komposisi; bukan nominal uang.
        const share = total > 0 ? (Number(row.net_sales) / total) * 100 : 0
        return <span className="num tabular-nums">{percent.format(share)}%</span>
      },
    },
  ]

  return (
    <>
      <PageHeader
        title="Laporan penjualan"
        description="Omzet dan jumlah pesanan selesai pada periode terpilih, dipecah per hari, jenis layanan, metode bayar, dan kasir."
      />

      <ReportShell kind="sales" from={from} to={to} onPeriodChange={setPeriod}>
        {report.isError ? (
          <Card>
            <div className="px-5 py-8 text-center">
              <p className="text-sm text-negative">{apiMessage(report.error)}</p>
            </div>
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4">
            <SummaryTiles data={data} loading={report.isLoading} />

            <Card>
              <CardHeader title="Penjualan per hari" />
              <DataTable
                columns={dayColumns}
                rows={activeDays}
                rowKey={(row) => row.key}
                loading={report.isLoading}
                empty="Belum ada pesanan selesai pada periode ini."
              />
            </Card>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <Card>
                <CardHeader title="Menurut jenis layanan" />
                <DataTable
                  columns={bucketColumns('Jenis layanan')}
                  rows={data?.by_service_type ?? []}
                  rowKey={(row) => row.key}
                  loading={report.isLoading}
                  empty="Belum ada data."
                />
              </Card>

              <Card>
                <CardHeader title="Menurut metode bayar" />
                <DataTable
                  columns={bucketColumns('Metode')}
                  rows={data?.by_payment_method ?? []}
                  rowKey={(row) => row.key}
                  loading={report.isLoading}
                  empty="Belum ada pembayaran."
                />
              </Card>

              <Card>
                <CardHeader title="Menurut kasir" />
                <DataTable
                  columns={bucketColumns('Kasir')}
                  rows={data?.by_cashier ?? []}
                  rowKey={(row) => row.key}
                  loading={report.isLoading}
                  empty="Belum ada data."
                />
              </Card>

              <Card>
                <CardHeader title="Menu terlaris" />
                <DataTable
                  columns={[
                    { key: 'name', header: 'Menu', render: (row) => row.name },
                    {
                      key: 'qty',
                      header: 'Terjual',
                      className: 'text-right',
                      render: (row) => row.quantity,
                    },
                    {
                      key: 'revenue',
                      header: 'Omzet',
                      className: 'text-right whitespace-nowrap',
                      render: (row) => (
                        <span className="num tabular-nums">{formatMoney(row.revenue)}</span>
                      ),
                    },
                  ]}
                  rows={data?.top_products ?? []}
                  rowKey={(row) => row.product_id}
                  loading={report.isLoading}
                  empty="Belum ada penjualan."
                />
              </Card>
            </div>
          </div>
        )}
      </ReportShell>
    </>
  )
}

function SummaryTiles({ data, loading }: { data?: SalesReport; loading: boolean }) {
  const tiles: { label: string; value?: string; hint?: string }[] = [
    { label: 'Penjualan kotor', value: data?.total.gross_sales },
    { label: 'Diskon', value: data?.total.discount },
    { label: 'Penjualan bersih', value: data?.total.net_sales },
    { label: 'Service charge', value: data?.total.service_charge },
    { label: 'Pajak', value: data?.total.tax },
    { label: 'Total ditagih', value: data?.total.total },
    {
      label: 'Pesanan',
      value: data ? String(data.total.order_count) : undefined,
      hint: 'selesai',
    },
    { label: 'Rata-rata / pesanan', value: data?.total.aov },
  ]

  return (
    <Card>
      <div className="grid grid-cols-2 divide-line sm:grid-cols-4 lg:grid-cols-8">
        {tiles.map((tile) => (
          <div key={tile.label} className="px-4 py-3">
            <p className="text-xs text-muted">{tile.label}</p>
            {loading || tile.value === undefined ? (
              <Skeleton className="mt-2 h-6 w-20" />
            ) : (
              <p className="num mt-1 text-[15px] leading-tight tabular-nums">
                {tile.label === 'Pesanan' ? tile.value : formatMoney(tile.value)}
              </p>
            )}
            {tile.hint && <p className="text-[11px] text-faint">{tile.hint}</p>}
          </div>
        ))}
      </div>
    </Card>
  )
}
