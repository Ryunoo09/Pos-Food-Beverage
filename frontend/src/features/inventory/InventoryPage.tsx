import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Search, SlidersHorizontal } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { stockApi } from '../../api/stock'
import type { StockLevel } from '../../api/stock'
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
import { formatMoney } from '../../utils/formatMoney'
import { AdjustStockDialog } from './AdjustStockDialog'

const perPage = 20

export function InventoryPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canAdjust = permissions.includes('inventory.adjust')

  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const lowOnly = params.get('low') === 'true'
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [search, setSearch] = useState(q)
  const [adjusting, setAdjusting] = useState<StockLevel | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const stock = useQuery({
    queryKey: ['stock', { q, lowOnly, page }],
    queryFn: async () =>
      (
        await stockApi.list({
          q: q || undefined,
          low: lowOnly || undefined,
          page,
          per_page: perPage,
        })
      ).data,
  })

  const adjust = useMutation({
    mutationFn: (input: { ingredientID: string; delta: string; reason: string; note: string }) =>
      stockApi.adjust(input.ingredientID, {
        quantity_delta: input.delta,
        reason: input.reason,
        note: input.note.trim() || null,
      }),
    onSuccess: async () => {
      setAdjusting(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['stock'] }),
        queryClient.invalidateQueries({ queryKey: ['ingredients'] }),
      ])
      toast.push('Stok disesuaikan.')
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

  const lowCount = (stock.data?.data ?? []).filter((row) => row.is_low).length

  const columns: Column<StockLevel>[] = [
    {
      key: 'ingredient',
      header: 'Bahan',
      render: (row) => (
        <div className="whitespace-nowrap">
          <p className="font-medium text-ink">{row.name}</p>
          <p className="num text-xs text-muted">{row.code}</p>
        </div>
      ),
    },
    {
      key: 'onhand',
      header: 'Saldo',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <div>
          <p className="num font-medium">
            {quantity(row.quantity_on_hand)} {row.base_unit_code}
          </p>
          <p className="text-xs text-muted">
            Min {quantity(row.min_stock)} {row.base_unit_code}
          </p>
        </div>
      ),
    },
    {
      key: 'value',
      header: 'Nilai stok',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <div>
          <p className="num">{row.stock_value ? formatMoney(row.stock_value) : '-'}</p>
          <p className="text-xs text-muted">
            {row.avg_unit_cost ? `${formatMoney(row.avg_unit_cost)}/unit` : '-'}
          </p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) =>
        row.is_low ? <Badge tone="red">Stok rendah</Badge> : <Badge tone="green">Aman</Badge>,
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <div className="flex justify-end gap-1">
          <Link
            to={`/ingredients/${row.ingredient_id}`}
            className="inline-flex min-h-11 items-center rounded-control px-3 text-sm font-medium text-ink hover:bg-raised"
          >
            Riwayat
          </Link>
          {canAdjust && (
            <Button variant="secondary" size="sm" onClick={() => setAdjusting(row)}>
              <SlidersHorizontal className="size-4" strokeWidth={1.5} />
              Sesuaikan
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Stok bahan"
        description="Saldo terkini per bahan pada outlet ini, dihitung dari pergerakan stok."
        action={
          <Button variant="secondary" onClick={() => setDetailOpen(true)}>
            Panduan penyesuaian
          </Button>
        }
      />

      <div className="mt-6 flex flex-wrap items-end gap-3">
        <form
          className="min-w-[16rem] flex-1"
          onSubmit={(event) => {
            event.preventDefault()
            patchParams({ q: search.trim() })
          }}
        >
          <Input
            aria-label="Cari bahan"
            placeholder="Cari nama atau kode bahan…"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            trailing={
              <button
                type="submit"
                aria-label="Cari"
                className="grid size-11 place-items-center text-muted hover:text-ink"
              >
                <Search className="size-4" strokeWidth={1.5} />
              </button>
            }
          />
        </form>
        <div className="w-full sm:w-52">
          <Select
            ariaLabel="Filter stok"
            value={lowOnly ? 'low' : 'all'}
            onValueChange={(value) => patchParams({ low: value === 'low' ? 'true' : '' })}
            options={[
              { value: 'all', label: 'Semua bahan' },
              { value: 'low', label: 'Stok rendah saja' },
            ]}
          />
        </div>
      </div>

      {lowOnly && lowCount > 0 && (
        <p className="mt-3 text-sm text-warning">
          {lowCount} bahan di halaman ini berada di bawah stok minimum.
        </p>
      )}

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={stock.data?.data ?? []}
          rowKey={(row) => row.ingredient_id}
          loading={stock.isLoading}
          error={stock.isError ? apiMessage(stock.error) : undefined}
          onRetry={() => stock.refetch()}
          empty="Belum ada bahan dengan stok tercatat."
        />
        <Pagination
          page={stock.data?.meta.page ?? page}
          totalPages={stock.data?.meta.total_pages ?? 1}
          total={stock.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      {adjusting && (
        <AdjustStockDialog
          stock={adjusting}
          open={adjusting !== null}
          loading={adjust.isPending}
          onOpenChange={(open) => {
            if (!open && !adjust.isPending) setAdjusting(null)
          }}
          onSubmit={(input) =>
            adjust.mutate({
              ingredientID: adjusting.ingredient_id,
              delta: input.delta,
              reason: input.reason,
              note: input.note,
            })
          }
        />
      )}

      <Dialog
        open={detailOpen}
        onOpenChange={setDetailOpen}
        title="Tentang penyesuaian stok"
        description="Penyesuaian manual menulis movement ADJUSTMENT dan mengubah saldo seketika."
        footer={<Button onClick={() => setDetailOpen(false)}>Mengerti</Button>}
      >
        <ul className="list-disc space-y-2 pl-5 text-sm text-muted">
          <li>Isi jumlah positif untuk menambah stok, negatif untuk mengurangi.</li>
          <li>Alasan wajib diisi dan tercatat di riwayat pergerakan bahan.</li>
          <li>Untuk selisih hitung fisik rutin, gunakan menu Opname agar tercatat sebagai sesi.</li>
        </ul>
      </Dialog>
    </>
  )
}

function quantity(value: string): string {
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return value
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 3 }).format(parsed)
}
