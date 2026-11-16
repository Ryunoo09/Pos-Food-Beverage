import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipboardCheck, Plus } from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { inventoryApi } from '../../api/inventory'
import { OPNAME_STATUSES, OPNAME_STATUS_LABELS, opnameApi } from '../../api/opname'
import type { Opname } from '../../api/opname'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { opnameStatusTone } from '../../utils/status'

const perPage = 20

export function OpnamePage() {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('stock_opname.create')

  const [params, setParams] = useSearchParams()
  const status = params.get('status') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [createOpen, setCreateOpen] = useState(false)
  const [scope, setScope] = useState<'ALL' | 'SELECTED'>('ALL')
  const [note, setNote] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [ingredientSearch, setIngredientSearch] = useState('')

  const list = useQuery({
    queryKey: ['opnames', { status, page }],
    queryFn: async () =>
      (await opnameApi.list({ status: status || undefined, page, per_page: perPage })).data,
  })

  const ingredients = useQuery({
    queryKey: ['ingredients', { active: true, q: '' }],
    queryFn: async () =>
      (await inventoryApi.ingredients.list({ is_active: true, per_page: 100 })).data.data,
    enabled: createOpen,
  })

  const create = useMutation({
    mutationFn: () =>
      opnameApi.create({
        scope,
        ingredient_ids: scope === 'SELECTED' ? selected : [],
        note: note.trim() || null,
      }),
    onSuccess: async (result) => {
      setCreateOpen(false)
      setNote('')
      setSelected([])
      await queryClient.invalidateQueries({ queryKey: ['opnames'] })
      toast.push('Sesi opname dibuat.')
      navigate(`/inventory/opname/${result.data.data.id}`)
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  function patchParams(patch: Record<string, string>) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    if (!('page' in patch)) next.delete('page')
    setParams(next, { replace: true })
  }

  const columns: Column<Opname>[] = [
    {
      key: 'code',
      header: 'Sesi',
      render: (row) => (
        <div className="whitespace-nowrap">
          <p className="num font-medium text-ink">{row.code}</p>
          <p className="text-xs text-muted">
            {row.scope === 'ALL' ? 'Semua bahan' : 'Bahan terpilih'} · {formatDate(row.created_at)}
          </p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={opnameStatusTone(row.status)}>
          {OPNAME_STATUS_LABELS[row.status] ?? row.status}
        </Badge>
      ),
    },
    {
      key: 'progress',
      header: 'Progres hitung',
      className: 'whitespace-nowrap',
      render: (row) => (
        <span className="num text-sm">
          {row.counted_items} / {row.total_items}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) => (
        <Link
          to={`/inventory/opname/${row.id}`}
          className="inline-flex min-h-11 items-center rounded-control px-3 text-sm font-medium text-ink hover:bg-raised"
        >
          Detail
        </Link>
      ),
    },
  ]

  const ingredientOptions = (ingredients.data ?? []).filter((item) => {
    const needle = ingredientSearch.trim().toLowerCase()
    if (needle === '') return true
    return (
      item.name.toLowerCase().includes(needle) || item.code.toLowerCase().includes(needle)
    )
  })

  return (
    <>
      <PageHeader
        title="Stock opname"
        description="Hitung fisik bahan, ajukan untuk disetujui, lalu stok disesuaikan otomatis."
        action={
          canCreate && (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" strokeWidth={2} />
              Sesi baru
            </Button>
          )
        }
      />

      <div className="mt-6 flex justify-end">
        <div className="w-full sm:w-60">
          <Select
            ariaLabel="Filter status opname"
            value={status || 'all'}
            onValueChange={(value) => patchParams({ status: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua status' },
              ...OPNAME_STATUSES.map((item) => ({
                value: item,
                label: OPNAME_STATUS_LABELS[item] ?? item,
              })),
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={list.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={list.isLoading}
          error={list.isError ? apiMessage(list.error) : undefined}
          onRetry={() => list.refetch()}
          empty="Belum ada sesi opname."
        />
        <Pagination
          page={list.data?.meta.page ?? page}
          totalPages={list.data?.meta.total_pages ?? 1}
          total={list.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      <Dialog
        open={createOpen}
        onOpenChange={(next) => !create.isPending && setCreateOpen(next)}
        title="Buat sesi opname"
        description="Sesi dimulai sebagai draft; saldo sistem baru dibekukan saat sesi dijalankan."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCreateOpen(false)} disabled={create.isPending}>
              Batal
            </Button>
            <Button
              loading={create.isPending}
              disabled={scope === 'SELECTED' && selected.length === 0}
              onClick={() => create.mutate()}
            >
              Buat sesi
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Select
            label="Cakupan"
            value={scope}
            onValueChange={(value) => setScope(value as 'ALL' | 'SELECTED')}
            options={[
              { value: 'ALL', label: 'Semua bahan aktif' },
              { value: 'SELECTED', label: 'Pilih bahan tertentu' },
            ]}
          />
          {scope === 'SELECTED' && (
            <div>
              <Input
                label="Cari bahan"
                placeholder="Nama atau kode bahan…"
                value={ingredientSearch}
                onChange={(event) => setIngredientSearch(event.target.value)}
              />
              <div className="mt-2 max-h-56 overflow-y-auto rounded-control border border-line">
                {ingredients.isLoading ? (
                  <p className="p-3 text-sm text-muted">Memuat bahan…</p>
                ) : ingredientOptions.length === 0 ? (
                  <p className="p-3 text-sm text-muted">Tidak ada bahan yang cocok.</p>
                ) : (
                  <ul className="divide-y divide-rule">
                    {ingredientOptions.map((ingredient) => (
                      <li key={ingredient.id}>
                        <label className="flex min-h-11 cursor-pointer items-center gap-3 px-3 text-sm hover:bg-raised">
                          <input
                            type="checkbox"
                            className="size-4"
                            checked={selected.includes(ingredient.id)}
                            onChange={(event) =>
                              setSelected((current) =>
                                event.target.checked
                                  ? [...current, ingredient.id]
                                  : current.filter((id) => id !== ingredient.id),
                              )
                            }
                          />
                          <span className="min-w-0 flex-1">
                            <span className="block truncate font-medium">{ingredient.name}</span>
                            <span className="num block text-xs text-muted">
                              {ingredient.code} · {ingredient.base_unit_code}
                            </span>
                          </span>
                        </label>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <p className="mt-2 text-xs text-muted">
                {selected.length} bahan dipilih.
              </p>
            </div>
          )}
          <Input
            label="Catatan"
            placeholder="Contoh: opname akhir bulan"
            value={note}
            onChange={(event) => setNote(event.target.value)}
          />
          {scope === 'ALL' && (
            <p className="flex items-start gap-2 text-sm text-muted">
              <ClipboardCheck className="mt-0.5 size-4 shrink-0" strokeWidth={1.5} />
              Semua bahan aktif akan ikut sesi ini; bahan yang sedang ada di sesi lain akan
              menolak pembuatan sesi.
            </p>
          )}
        </div>
      </Dialog>
    </>
  )
}
