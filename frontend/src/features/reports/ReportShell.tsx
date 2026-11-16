import { useMutation } from '@tanstack/react-query'
import { Download, Info } from 'lucide-react'
import type { ReactNode } from 'react'
import { DISCLAIMER, reportApi } from '../../api/reports'
import type { ReportKind } from '../../api/reports'
import { Button } from '../../components/ui/Button'
import { DateRangePicker } from '../../components/ui/DateRangePicker'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'

/**
 * Kerangka halaman laporan: pemilih periode, tombol ekspor CSV, dan
 * disclaimer. Disclaimer selalu tampil karena angka HPP/laba adalah estimasi.
 */
export function ReportShell({
  kind,
  from,
  to,
  onPeriodChange,
  children,
}: {
  kind: ReportKind
  from: string
  to: string
  onPeriodChange: (next: { from: string; to: string }) => void
  children: ReactNode
}) {
  const toast = useToast()
  const permissions = useAuthStore((state) => state.permissions)
  const canExport = permissions.includes('reports.export')

  const exporter = useMutation({
    mutationFn: () => reportApi.exportCsv(kind, { from, to }),
    onSuccess: (response) => {
      const url = URL.createObjectURL(response.data)
      const link = document.createElement('a')
      link.href = url
      link.download = `${kind}_${from}_${to}.csv`
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
      toast.push('Ekspor CSV diunduh.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  return (
    <>
      <div className="mt-6 flex flex-wrap items-end justify-between gap-3">
        <div className="w-full sm:w-auto">
          <DateRangePicker value={{ from, to }} onChange={onPeriodChange} label="Periode laporan" />
        </div>
        {canExport && (
          <Button
            variant="secondary"
            onClick={() => exporter.mutate()}
            loading={exporter.isPending}
          >
            <Download className="size-4" strokeWidth={1.5} />
            Ekspor CSV
          </Button>
        )}
      </div>

      <p className="mt-3 flex items-start gap-2 rounded-control bg-raised px-3 py-2 text-xs text-muted">
        <Info className="mt-px size-3.5 shrink-0" strokeWidth={1.5} aria-hidden="true" />
        {DISCLAIMER}
      </p>

      <div className="mt-4">{children}</div>
    </>
  )
}
