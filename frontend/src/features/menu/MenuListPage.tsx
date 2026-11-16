import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Ban, Pencil, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { menuApi } from '../../api/menu'
import type { Product } from '../../api/menu'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatMoney } from '../../utils/formatMoney'

const perPage = 20

export function MenuListPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canManage = useAuthStore((state) => state.hasPermission('products.update'))
  const canSetAvailability = useAuthStore((state) => state.hasPermission('products.set_availability'))

  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const categoryID = params.get('category_id') ?? ''
  const availability = params.get('available') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [search, setSearch] = useState(q)
  const [deactivating, setDeactivating] = useState<Product | null>(null)

  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: async () => (await menuApi.categories.list()).data.data,
  })

  const products = useQuery({
    queryKey: ['products', { q, categoryID, availability, page }],
    queryFn: async () =>
      (
        await menuApi.products.list({
          q: q || undefined,
          category_id: categoryID || undefined,
          available: availability === '' ? undefined : availability === 'true',
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

  const toggle = useMutation({
    mutationFn: (product: Product) =>
      menuApi.products.setAvailability(product.id, !product.is_available),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['products'] })
      toast.push('Ketersediaan menu diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const deactivate = useMutation({
    mutationFn: (id: string) => menuApi.products.deactivate(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['products'] })
      toast.push('Menu dinonaktifkan. Riwayat pesanan tetap tersimpan.')
      setDeactivating(null)
    },
    onError: (error) => {
      toast.push(apiMessage(error), 'error')
      setDeactivating(null)
    },
  })

  const columns: Column<Product>[] = [
    {
      key: 'name',
      header: 'Menu',
      render: (row) => (
        <div className="min-w-[12rem]">
          <p className="font-medium text-ink">{row.name}</p>
          <p className="num text-xs text-muted">
            {row.sku} · {row.category_name}
          </p>
        </div>
      ),
    },
    {
      key: 'price',
      header: 'Harga',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.price)}</span>,
    },
    {
      key: 'availability',
      header: 'Tersedia',
      render: (row) => (
        <button
          type="button"
          role="switch"
          aria-checked={row.is_available}
          aria-label={`Ketersediaan ${row.name}`}
          disabled={!canSetAvailability || toggle.isPending}
          onClick={() => toggle.mutate(row)}
          className={cn(
            'inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm',
            canSetAvailability ? 'hover:bg-raised' : 'cursor-default',
            'disabled:opacity-60',
          )}
        >
          <span
            className={cn(
              'relative h-6 w-11 rounded-full transition-colors',
              row.is_available ? 'bg-positive' : 'bg-control',
            )}
          >
            <span
              className={cn(
                'absolute top-0.5 size-5 rounded-full bg-white transition-[left]',
                row.is_available ? 'left-[1.4rem]' : 'left-0.5',
              )}
            />
          </span>
          <span className={row.is_available ? 'text-ink' : 'text-muted'}>
            {row.is_available ? 'Ada' : 'Habis'}
          </span>
        </button>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={row.is_active ? 'green' : 'gray'}>{row.is_active ? 'Aktif' : 'Nonaktif'}</Badge>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-24 text-right',
      render: (row) =>
        canManage ? (
          <div className="flex justify-end gap-1">
            <Link
              to={`/menu/${row.id}`}
              className="inline-flex min-h-11 items-center gap-1 rounded-control px-3 text-sm font-medium text-ink hover:bg-raised"
            >
              <Pencil className="size-4" strokeWidth={1.5} />
              Ubah
            </Link>
            {row.is_active && (
              <Button
                variant="ghost"
                size="sm"
                className="text-negative"
                onClick={() => setDeactivating(row)}
                aria-label={`Nonaktifkan ${row.name}`}
              >
                <Ban className="size-4" strokeWidth={1.5} />
              </Button>
            )}
          </div>
        ) : null,
    },
  ]

  return (
    <>
      <PageHeader
        title="Menu"
        description="Daftar hidangan dan minuman yang bisa dipesan pelanggan."
        action={
          canManage && (
            <Link to="/menu/new">
              <Button>
                <Plus className="size-4" strokeWidth={2} />
                Tambah menu
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
            aria-label="Cari menu"
            placeholder="Cari nama atau SKU…"
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
        <div className="w-full sm:w-56">
          <Select
            ariaLabel="Filter kategori"
            value={categoryID}
            onValueChange={(value) => patchParams({ category_id: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua kategori' },
              ...(categories.data ?? []).map((category) => ({
                value: category.id,
                label: category.name,
              })),
            ]}
          />
        </div>
        <div className="w-full sm:w-44">
          <Select
            ariaLabel="Filter ketersediaan"
            value={availability || 'all'}
            onValueChange={(value) => patchParams({ available: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua status' },
              { value: 'true', label: 'Tersedia' },
              { value: 'false', label: 'Habis' },
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={products.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={products.isLoading}
          error={products.isError ? apiMessage(products.error) : undefined}
          onRetry={() => products.refetch()}
          empty="Tidak ada menu yang cocok dengan filter."
        />
        <Pagination
          page={products.data?.meta.page ?? page}
          totalPages={products.data?.meta.total_pages ?? 1}
          total={products.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      <ConfirmDialog
        open={deactivating !== null}
        title="Nonaktifkan menu?"
        description={`"${deactivating?.name ?? ''}" tidak akan muncul di kasir. Menu tetap tersimpan dan bisa diaktifkan lagi.`}
        confirmLabel="Nonaktifkan"
        loading={deactivate.isPending}
        onConfirm={() => deactivating && deactivate.mutate(deactivating.id)}
        onCancel={() => setDeactivating(null)}
      />
    </>
  )
}
