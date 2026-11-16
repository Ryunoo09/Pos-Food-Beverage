import { useQuery } from '@tanstack/react-query'
import { Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { inventoryApi } from '../../api/inventory'
import type { Ingredient } from '../../api/inventory'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

const perPage = 20

export function IngredientsPage() {
  const canManage = useAuthStore((state) => state.hasPermission('ingredients.update'))
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const active = params.get('is_active') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [search, setSearch] = useState(q)

  const ingredients = useQuery({
    queryKey: ['ingredients', { q, active, page }],
    queryFn: async () =>
      (
        await inventoryApi.ingredients.list({
          q: q || undefined,
          is_active: active === '' ? undefined : active === 'true',
          page,
          per_page: perPage,
        })
      ).data,
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

  const columns: Column<Ingredient>[] = [
    {
      key: 'name',
      header: 'Bahan',
      render: (row) => (
        <div className="min-w-[12rem]">
          <Link to={`/ingredients/${row.id}`} className="font-medium text-ink hover:underline">
            {row.name}
          </Link>
          <p className="num text-xs text-muted">
            {row.code} · {row.category_name ?? 'Tanpa kategori'}
          </p>
        </div>
      ),
    },
    {
      key: 'stock',
      header: 'Stok saat ini',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num">
          {quantity(row.quantity_on_hand)} {row.base_unit_code}
        </span>
      ),
    },
    {
      key: 'min',
      header: 'Minimum',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num text-muted">
          {quantity(row.min_stock)} {row.base_unit_code}
        </span>
      ),
    },
    {
      key: 'cost',
      header: 'Biaya rata-rata',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.avg_unit_cost)}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => {
        const state = stockState(row)
        return <Badge tone={state.tone}>{state.label}</Badge>
      },
    },
  ]

  return (
    <>
      <PageHeader
        title="Bahan"
        description="Bahan baku yang dipakai resep, beserta stok dan biaya rata-ratanya."
        action={
          canManage && (
            <Link to="/ingredients/new">
              <Button>
                <Plus className="size-4" strokeWidth={2} />
                Tambah bahan
              </Button>
            </Link>
          )
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
        <div className="w-full sm:w-44">
          <Select
            aria-label="Filter status bahan"
            value={active || 'all'}
            onValueChange={(value) => patchParams({ is_active: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua bahan' },
              { value: 'true', label: 'Aktif' },
              { value: 'false', label: 'Nonaktif' },
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={ingredients.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={ingredients.isLoading}
          error={ingredients.isError ? apiMessage(ingredients.error) : undefined}
          onRetry={() => ingredients.refetch()}
          empty="Belum ada bahan. Tambahkan bahan pertama."
        />
        <Pagination
          page={ingredients.data?.meta.page ?? page}
          totalPages={ingredients.data?.meta.total_pages ?? 1}
          total={ingredients.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>
    </>
  )
}

function stockState(ingredient: Ingredient): {
  label: string
  tone: 'green' | 'yellow' | 'red' | 'gray'
} {
  const onHand = Number(ingredient.quantity_on_hand)
  const minimum = Number(ingredient.min_stock)
  if (onHand <= 0) return { label: 'Habis', tone: 'red' }
  if (minimum > 0 && onHand <= minimum * 0.5) return { label: 'Kritis', tone: 'red' }
  if (minimum > 0 && onHand <= minimum) return { label: 'Rendah', tone: 'yellow' }
  return { label: 'Aman', tone: 'green' }
}

function quantity(value: string): string {
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 2 }).format(Number(value))
}
