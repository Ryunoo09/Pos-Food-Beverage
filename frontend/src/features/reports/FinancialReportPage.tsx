import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { reportApi } from '../../api/reports'
import type { FinancialReport } from '../../api/reports'
import { PageHeader } from '../../components/shared/PageHeader'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import { Skeleton } from '../../components/ui/Skeleton'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatMoney } from '../../utils/formatMoney'
import { useReportPeriod } from './period'
import { ReportShell } from './ReportShell'

export function FinancialReportPage() {
  const { from, to, setPeriod } = useReportPeriod()

  const report = useQuery({
    queryKey: ['report-financial', { from, to }],
    queryFn: async () => (await reportApi.financial({ from, to })).data.data,
  })

  const data = report.data

  return (
    <>
      <PageHeader
        title="Laporan keuangan"
        description="Estimasi laba rugi dari penjualan selesai, HPP tersimpan, kerugian waste, dan pengeluaran operasional pada periode terpilih."
      />

      <ReportShell kind="financial" from={from} to={to} onPeriodChange={setPeriod}>
        {report.isError ? (
          <Card>
            <div className="px-5 py-8 text-center">
              <p className="text-sm text-negative">{apiMessage(report.error)}</p>
            </div>
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
            <Card className="lg:col-span-7">
              <CardHeader title="Laba rugi" />
              <ProfitStatement data={data} loading={report.isLoading} />
            </Card>

            <Card className="lg:col-span-5">
              <CardHeader title="Pengeluaran per kategori" />
              <DataTable
                columns={[
                  { key: 'category', header: 'Kategori', render: (row) => row.category_name },
                  {
                    key: 'amount',
                    header: 'Nominal',
                    className: 'text-right whitespace-nowrap',
                    render: (row) => (
                      <span className="num tabular-nums">{formatMoney(row.amount)}</span>
                    ),
                  },
                ]}
                rows={data?.expenses_by_category ?? []}
                rowKey={(row) => row.category_name}
                loading={report.isLoading}
                empty="Belum ada pengeluaran operasional."
              />
            </Card>
          </div>
        )}
      </ReportShell>
    </>
  )
}

/**
 * Baris laporan disusun sebagai daftar baris berlabel, bukan tabel, karena
 * hubungan antar-baris (subtotal, hasil bagi) lebih penting daripada kolom.
 */
function ProfitStatement({ data, loading }: { data?: FinancialReport; loading: boolean }) {
  if (loading || !data) {
    return (
      <div className="space-y-3 p-5">
        {Array.from({ length: 7 }, (_, index) => (
          <Skeleton key={index} className="h-6 w-full" />
        ))}
      </div>
    )
  }

  const revenue: { label: string; value: string; muted?: boolean }[] = [
    { label: 'Penjualan kotor', value: data.gross_sales },
    { label: 'Diskon', value: data.discount, muted: true },
    { label: 'Service charge', value: data.service_charge, muted: true },
    { label: 'Pajak', value: data.tax, muted: true },
  ]

  return (
    <div className="p-5">
      <StatementGroup title="Pendapatan">
        {revenue.map((row) => (
          <StatementRow key={row.label} label={row.label} value={row.value} muted={row.muted} />
        ))}
        <StatementRow label="Penjualan bersih" value={data.net_sales} emphasis />
      </StatementGroup>

      {data.hpp === undefined ? (
        <p className="mt-4 rounded-control bg-raised px-3 py-2 text-xs text-muted">
          Rincian HPP dan laba hanya tersedia untuk pengguna dengan akses laporan laba.
        </p>
      ) : (
        <>
          <StatementGroup title="Biaya pokok">
            <StatementRow label="HPP (Harga Pokok Penjualan)" value={data.hpp} muted />
            <StatementRow label="Laba kotor" value={data.gross_profit ?? '0'} emphasis />
            {data.gross_margin_percent !== undefined && (
              <StatementRow
                label="Margin kotor"
                value={`${Number(data.gross_margin_percent).toLocaleString('id-ID', {
                  maximumFractionDigits: 2,
                })}%`}
                suffixOnly
              />
            )}
          </StatementGroup>

          <StatementGroup title="Biaya operasional">
            <StatementRow label="Kerugian waste" value={data.waste ?? '0'} muted />
            <StatementRow
              label="Pengeluaran operasional"
              value={data.operating_expense ?? '0'}
              muted
            />
            <StatementRow
              label="Estimasi laba bersih"
              value={data.net_profit ?? '0'}
              emphasis
              negative={data.net_profit?.startsWith('-')}
            />
          </StatementGroup>
        </>
      )}
    </div>
  )
}

function StatementGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-5 first:mt-0">
      <h3 className="mb-1 text-xs font-medium tracking-wide text-faint uppercase">{title}</h3>
      <dl className="divide-y divide-rule">{children}</dl>
    </section>
  )
}

function StatementRow({
  label,
  value,
  muted,
  emphasis,
  negative,
  suffixOnly,
}: {
  label: string
  value: string
  muted?: boolean
  emphasis?: boolean
  negative?: boolean
  suffixOnly?: boolean
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2">
      <dt className={cn('text-sm', muted ? 'text-muted' : 'text-ink')}>{label}</dt>
      <dd
        className={cn(
          'num shrink-0 text-sm whitespace-nowrap tabular-nums',
          emphasis && 'font-medium',
          negative ? 'text-negative' : 'text-ink',
        )}
      >
        {suffixOnly ? value : formatMoney(value)}
      </dd>
    </div>
  )
}
