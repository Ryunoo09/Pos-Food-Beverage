import { useQuery } from '@tanstack/react-query'
import { Download } from 'lucide-react'
import { useState } from 'react'
import { auditApi } from '../../api/audit'
import type { AuditFilter } from '../../api/audit'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'

const PER_PAGE = 25

export function AuditLogPage() {
  const toast = useToast()
  const permissions = useAuthStore((state) => state.permissions)
  const canExport = permissions.includes('audit.export')

  const [page, setPage] = useState(1)
  const [action, setAction] = useState('')
  const [detail, setDetail] = useState<number | null>(null)
  const [exporting, setExporting] = useState(false)

  const filter: AuditFilter = {
    page,
    per_page: PER_PAGE,
    ...(action.trim() !== '' ? { action: action.trim() } : {}),
  }

  const logs = useQuery({
    queryKey: ['audit-logs', filter.page, filter.action],
    queryFn: async () => (await auditApi.list(filter)).data,
  })

  async function handleExport() {
    setExporting(true)
    try {
      const response = await auditApi.exportCsv({ ...filter, page: 1, per_page: 100 })
      // Pakai blob + object URL agar header Authorization ikut terkirim;
      // <a href> biasa tidak membawa token.
      const url = URL.createObjectURL(response.data)
      const link = document.createElement('a')
      link.href = url
      link.download = 'audit-logs.csv'
      link.click()
      URL.revokeObjectURL(url)
      toast.push('Ekspor diunduh.')
    } catch (error) {
      toast.push(apiMessage(error), 'error')
    } finally {
      setExporting(false)
    }
  }

  const rows = logs.data?.data ?? []
  const meta = logs.data?.meta
  const selected = rows.find((log) => log.id === detail)

  return (
    <>
      <PageHeader
        title="Log audit"
        description="Catatan siapa melakukan apa dan kapan. Dipakai untuk menelusuri perubahan data penting."
        action={
          canExport && (
            <Button
              size="sm"
              variant="secondary"
              loading={exporting}
              onClick={handleExport}
            >
              <Download className="size-4" strokeWidth={2} />
              Ekspor CSV
            </Button>
          )
        }
      />

      <Card className="mt-6">
        <CardHeader
          title="Aktivitas"
          action={
            <div className="w-56">
              <Input
                placeholder="Saring aksi, mis. payment.create"
                value={action}
                onChange={(event) => {
                  setAction(event.target.value)
                  setPage(1)
                }}
              />
            </div>
          }
        />
        {logs.isLoading ? (
          <div className="space-y-3 p-5">
            {Array.from({ length: 6 }, (_, index) => (
              <Skeleton key={index} className="h-10 w-full" />
            ))}
          </div>
        ) : logs.isError ? (
          <p className="px-5 py-8 text-sm text-negative">{apiMessage(logs.error)}</p>
        ) : rows.length === 0 ? (
          <p className="px-5 py-10 text-center text-sm text-muted">Tidak ada aktivitas tercatat.</p>
        ) : (
          <ul className="divide-y divide-rule">
            {rows.map((log) => (
              <li key={log.id}>
                <button
                  type="button"
                  onClick={() => setDetail(log.id)}
                  className="flex min-h-11 w-full flex-wrap items-center gap-3 px-5 py-3 text-left hover:bg-raised"
                >
                  <span className="num w-40 shrink-0 text-xs text-muted">
                    {formatTimestamp(log.occurred_at)}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="num text-[13px] text-ink">{log.action}</span>
                      {log.entity_type && <Badge tone="gray">{log.entity_type}</Badge>}
                      {log.user_roles && log.user_roles.length > 0 && (
                        <Badge tone="blue">{log.user_roles.join(', ')}</Badge>
                      )}
                    </div>
                    {log.reason && <p className="mt-0.5 text-xs text-muted">{log.reason}</p>}
                  </div>
                </button>
              </li>
            ))}
          </ul>
        )}
        {meta && (
          <Pagination
            page={meta.page}
            totalPages={meta.total_pages}
            total={meta.total}
            onPageChange={setPage}
          />
        )}
      </Card>

      {selected && (
        <Dialog
          open
          onOpenChange={(open) => !open && setDetail(null)}
          title={selected.action}
          description={formatTimestamp(selected.occurred_at)}
          footer={
            <Button variant="secondary" onClick={() => setDetail(null)}>
              Tutup
            </Button>
          }
        >
          <dl className="space-y-3 text-sm">
            <Row label="Pengguna" value={selected.user_id ?? '—'} />
            <Row label="Entitas" value={selected.entity_type ?? '—'} />
            <Row label="ID entitas" value={selected.entity_id ?? '—'} />
            <Row label="Alasan" value={selected.reason ?? '—'} />
            <Row label="Alamat IP" value={selected.ip_address ?? '—'} />
            <Row label="Request ID" value={selected.request_id ?? '—'} />
            {selected.before_data !== undefined && (
              <Row label="Sebelum" value={JSON.stringify(selected.before_data)} />
            )}
            {selected.after_data !== undefined && (
              <Row label="Sesudah" value={JSON.stringify(selected.after_data)} />
            )}
          </dl>
        </Dialog>
      )}
    </>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[7rem_1fr] gap-2">
      <dt className="text-muted">{label}</dt>
      <dd className="num break-all">{value}</dd>
    </div>
  )
}

/** Waktu lokal Jakarta: "16 Nov 2026, 14.05". */
function formatTimestamp(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'Asia/Jakarta',
  }).format(date)
}
