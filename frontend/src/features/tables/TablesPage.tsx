import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2, Users } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { tableApi, TABLE_STATUSES } from '../../api/tables'
import type { Table, TableInput, TableStatus } from '../../api/tables'
import { PageHeader } from '../../components/shared/PageHeader'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'

const STATUS: Record<TableStatus, { label: string; className: string; dot: string }> = {
  AVAILABLE: {
    label: 'Kosong',
    className: 'border-positive/40 bg-pill-positive/30',
    dot: 'bg-positive',
  },
  OCCUPIED: {
    label: 'Terisi',
    className: 'border-negative/40 bg-pill-negative/40',
    dot: 'bg-negative',
  },
  RESERVED: {
    label: 'Reservasi',
    className: 'border-chart-2/40 bg-pill-info/40',
    dot: 'bg-chart-2',
  },
  CLEANING: {
    label: 'Dibersihkan',
    className: 'border-warning/40 bg-pill-warning/40',
    dot: 'bg-warning',
  },
}

interface TableDraft {
  id?: string
  number: string
  area: string
  capacity: string
  sort_order: number
}

const emptyDraft: TableDraft = { number: '', area: '', capacity: '', sort_order: 0 }

export function TablesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canManage = useAuthStore((state) => state.hasPermission('tables.manage'))
  const [area, setArea] = useState('all')
  const [draft, setDraft] = useState<TableDraft | null>(null)
  const [deleting, setDeleting] = useState<Table | null>(null)
  const [error, setError] = useState<string | null>(null)

  const tables = useQuery({
    queryKey: ['tables'],
    queryFn: async () => (await tableApi.list()).data.data,
  })

  const areas = [...new Set((tables.data ?? []).map((table) => table.area ?? 'Tanpa area'))]
  const visible = (tables.data ?? []).filter(
    (table) => area === 'all' || (table.area ?? 'Tanpa area') === area,
  )
  const grouped = visible.reduce<Record<string, Table[]>>((result, table) => {
    const key = table.area ?? 'Tanpa area'
    ;(result[key] ??= []).push(table)
    return result
  }, {})

  const save = useMutation({
    mutationFn: (values: TableDraft) => {
      const body: TableInput = {
        number: values.number.trim(),
        area: values.area.trim() || null,
        capacity: values.capacity ? Number(values.capacity) : null,
        sort_order: values.sort_order,
      }
      return values.id ? tableApi.update(values.id, body) : tableApi.create(body)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      toast.push(draft?.id ? 'Meja diperbarui.' : 'Meja dibuat.')
      setDraft(null)
    },
    onError: (err) => setError(apiMessage(err)),
  })

  const setStatus = useMutation({
    mutationFn: ({ table, status }: { table: Table; status: TableStatus }) =>
      tableApi.setStatus(table.id, status),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      toast.push('Status meja diperbarui.')
    },
    onError: (err) => toast.push(apiMessage(err), 'error'),
  })

  const remove = useMutation({
    mutationFn: (id: string) => tableApi.remove(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      toast.push('Meja dihapus.')
      setDeleting(null)
    },
    onError: (err) => {
      toast.push(apiMessage(err), 'error')
      setDeleting(null)
    },
  })

  const counts = TABLE_STATUSES.map((status) => ({
    status,
    count: (tables.data ?? []).filter((table) => table.status === status).length,
  }))

  return (
    <>
      <PageHeader
        title="Denah meja"
        description="Pilih meja kosong untuk mulai pesanan pelanggan, atau ubah status saat meja dibersihkan."
        action={
          canManage && (
            <Button
              onClick={() => {
                setError(null)
                setDraft({ ...emptyDraft })
              }}
            >
              <Plus className="size-4" strokeWidth={2} />
              Tambah meja
            </Button>
          )
        }
      />

      <div className="mt-6 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2">
          {counts.map(({ status, count }) => (
            <span
              key={status}
              className="inline-flex min-h-9 items-center gap-2 rounded-control border border-line bg-surface px-3 text-sm"
            >
              <span className={cn('size-2 rounded-full', STATUS[status].dot)} />
              {STATUS[status].label}
              <span className="num text-muted">{count}</span>
            </span>
          ))}
        </div>
        <div className="w-full sm:w-52">
          <Select
            ariaLabel="Filter area"
            value={area}
            onValueChange={setArea}
            options={[
              { value: 'all', label: 'Semua area' },
              ...areas.map((item) => ({ value: item, label: item })),
            ]}
          />
        </div>
      </div>

      {tables.isLoading ? (
        <div className="mt-6 grid grid-cols-2 gap-4 md:grid-cols-4">
          {Array.from({ length: 8 }, (_, index) => (
            <Skeleton key={index} className="h-40 w-full" />
          ))}
        </div>
      ) : tables.isError ? (
        <Card className="mt-6 p-6 text-sm text-negative">{apiMessage(tables.error)}</Card>
      ) : visible.length === 0 ? (
        <Card className="mt-6 p-10 text-center text-sm text-muted">
          Belum ada meja di area ini. Tambahkan meja pertama.
        </Card>
      ) : (
        <div className="mt-6 space-y-8">
          {Object.entries(grouped).map(([areaName, rows]) => (
            <section key={areaName}>
              <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-ink">
                {areaName}
                <span className="text-xs font-normal text-muted">{rows.length} meja</span>
              </h2>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                {rows.map((table) => (
                  <TableCard
                    key={table.id}
                    table={table}
                    canManage={canManage}
                    busy={setStatus.isPending}
                    onStatus={(status) => setStatus.mutate({ table, status })}
                    onEdit={() => {
                      setError(null)
                      setDraft({
                        id: table.id,
                        number: table.number,
                        area: table.area ?? '',
                        capacity: table.capacity ? String(table.capacity) : '',
                        sort_order: table.sort_order,
                      })
                    }}
                    onDelete={() => setDeleting(table)}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      <Dialog
        open={draft !== null}
        onOpenChange={(open) => !save.isPending && !open && setDraft(null)}
        title={draft?.id ? 'Ubah meja' : 'Meja baru'}
        description="Nama area dipakai untuk mengelompokkan denah. Nomor meja harus unik."
        footer={
          <>
            <Button variant="secondary" onClick={() => setDraft(null)} disabled={save.isPending}>
              Batal
            </Button>
            <Button
              onClick={() => draft && save.mutate(draft)}
              loading={save.isPending}
              disabled={!draft?.number.trim()}
            >
              Simpan
            </Button>
          </>
        }
      >
        {draft && (
          <div className="space-y-4">
            <Input
              label="Nomor meja"
              placeholder="Contoh: M5"
              autoFocus
              value={draft.number}
              onChange={(event) => setDraft({ ...draft, number: event.target.value })}
            />
            <Input
              label="Area"
              placeholder="Contoh: Indoor, Teras, Lantai 2"
              value={draft.area}
              onChange={(event) => setDraft({ ...draft, area: event.target.value })}
            />
            <Input
              label="Kapasitas kursi"
              type="number"
              min={1}
              placeholder="4"
              value={draft.capacity}
              onChange={(event) => setDraft({ ...draft, capacity: event.target.value })}
            />
            <Input
              label="Urutan tampil"
              type="number"
              min={0}
              value={draft.sort_order}
              onChange={(event) =>
                setDraft({ ...draft, sort_order: Math.max(0, Number(event.target.value) || 0) })
              }
            />
            {error && (
              <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
                {error}
              </p>
            )}
          </div>
        )}
      </Dialog>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus meja?"
        description={`Meja ${deleting?.number ?? ''} akan dihapus. Meja dengan pesanan aktif tidak bisa dihapus.`}
        confirmLabel="Hapus"
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onCancel={() => setDeleting(null)}
      />
    </>
  )
}

function TableCard({
  table,
  canManage,
  busy,
  onStatus,
  onEdit,
  onDelete,
}: {
  table: Table
  canManage: boolean
  busy: boolean
  onStatus: (status: TableStatus) => void
  onEdit: () => void
  onDelete: () => void
}) {
  const tone = STATUS[table.status]
  const available = table.status === 'AVAILABLE'

  return (
    <article
      className={cn(
        'flex min-h-44 flex-col rounded-card border p-4 text-left',
        tone.className,
        available && 'hover:border-positive',
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs text-muted">Meja</p>
          <p className="num text-2xl leading-tight font-semibold break-all text-ink sm:text-3xl">
            {table.number}
          </p>
        </div>
        <span className="inline-flex shrink-0 items-center gap-1.5 rounded-chip bg-surface/80 px-2 py-1 text-[11px] font-medium whitespace-nowrap text-ink">
          <span className={cn('size-1.5 rounded-full', tone.dot)} />
          {tone.label}
        </span>
      </div>

      <p className="mt-2 flex items-center gap-1 text-xs text-muted">
        <Users className="size-3.5" strokeWidth={1.5} />
        {table.capacity ? `${table.capacity} kursi` : 'Kapasitas belum diisi'}
      </p>

      <div className="mt-auto flex flex-wrap items-end justify-between gap-2 pt-4">
        {available ? (
          <Link
            to={`/pos?table=${table.id}`}
            className="inline-flex min-h-11 flex-1 items-center justify-center rounded-control bg-ink px-3 text-sm font-medium text-surface hover:opacity-90"
          >
            Mulai pesanan
          </Link>
        ) : (
          <span className="flex-1 text-xs text-muted">
            {table.status === 'OCCUPIED'
              ? 'Pesanan berjalan. Selesaikan pembayaran sebelum meja dikosongkan.'
              : 'Ubah status setelah meja siap dipakai.'}
          </span>
        )}
      </div>

      {canManage && (
        <div className="mt-2 flex items-center gap-1 border-t border-ink/10 pt-2">
          <div className="min-w-0 flex-1">
            <Select
              ariaLabel={`Status meja ${table.number}`}
              value={table.status}
              onValueChange={(value) => onStatus(value as TableStatus)}
              disabled={busy}
              options={TABLE_STATUSES.map((status) => ({
                value: status,
                label: STATUS[status].label,
              }))}
            />
          </div>
          <Button variant="ghost" size="sm" onClick={onEdit} aria-label={`Ubah meja ${table.number}`}>
            <Pencil className="size-4" strokeWidth={1.5} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-negative"
            onClick={onDelete}
            aria-label={`Hapus meja ${table.number}`}
          >
            <Trash2 className="size-4" strokeWidth={1.5} />
          </Button>
        </div>
      )}
    </article>
  )
}
