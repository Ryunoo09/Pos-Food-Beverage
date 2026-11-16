import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { menuApi } from '../../api/menu'
import type { Product, Variant, VariantInput } from '../../api/menu'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

interface Draft {
  id?: string
  name: string
  sku: string
  price: string
  is_default: boolean
  is_active: boolean
  sort_order: number
}

const newDraft: Draft = {
  name: '',
  sku: '',
  price: '0',
  is_default: false,
  is_active: true,
  sort_order: 0,
}

export function VariantsTab({ product, onChanged }: { product: Product; onChanged: () => void }) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canSetPrice = useAuthStore((state) => state.hasPermission('products.update_price'))
  const variants = product.variants ?? []

  const [draft, setDraft] = useState<Draft | null>(null)
  const [historyVariant, setHistoryVariant] = useState(
    variants.find((variant) => variant.is_default)?.id ?? variants[0]?.id ?? '',
  )
  const [error, setError] = useState<string | null>(null)

  const save = useMutation({
    mutationFn: async (values: Draft) => {
      const body: VariantInput = {
        name: values.name.trim(),
        sku: values.sku.trim(),
        price: values.price,
        is_default: values.is_default,
        is_active: values.is_active,
        sort_order: values.sort_order,
      }
      const currentDefault = variants.find(
        (variant) => variant.is_default && variant.id !== values.id,
      )
      if (body.is_default && currentDefault) {
        await menuApi.variants.update(currentDefault.id, toInput(currentDefault, false))
      }
      return values.id
        ? menuApi.variants.update(values.id, body)
        : menuApi.variants.create(product.id, body)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['product', product.id] })
      await queryClient.invalidateQueries({ queryKey: ['products'] })
      onChanged()
      toast.push(draft?.id ? 'Varian diperbarui.' : 'Varian ditambahkan.')
      setDraft(null)
    },
    onError: (err) => setError(apiMessage(err)),
  })

  const history = useQuery({
    queryKey: ['price-history', historyVariant],
    queryFn: async () => (await menuApi.variants.priceHistory(historyVariant)).data.data,
    enabled: historyVariant !== '',
  })

  const columns: Column<Variant>[] = [
    {
      key: 'name',
      header: 'Varian',
      render: (row) => (
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium text-ink">{row.name}</span>
          {row.is_default && <Badge tone="blue">Default</Badge>}
          {!row.is_active && <Badge tone="gray">Nonaktif</Badge>}
          <span className="num text-xs text-muted">{row.sku}</span>
        </div>
      ),
    },
    {
      key: 'price',
      header: 'Harga jual',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.price)}</span>,
    },
    {
      key: 'actions',
      header: '',
      className: 'w-56 text-right',
      render: (row) => (
        <div className="flex justify-end gap-1">
          <Link
            to={`/recipes/${row.id}`}
            className="inline-flex min-h-11 items-center rounded-control px-3 text-sm text-muted hover:bg-raised hover:text-ink"
          >
            Resep
          </Link>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setError(null)
              setDraft({
                id: row.id,
                name: row.name,
                sku: row.sku ?? '',
                price: wholeRupiah(row.price),
                is_default: row.is_default,
                is_active: row.is_active,
                sort_order: row.sort_order,
              })
            }}
            aria-label={`Ubah varian ${row.name}`}
          >
            <Pencil className="size-4" strokeWidth={1.5} />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader
          title="Varian dan harga"
          action={
            <Button
              size="sm"
              onClick={() => {
                setError(null)
                setDraft({ ...newDraft, sort_order: variants.length })
              }}
            >
              <Plus className="size-4" strokeWidth={2} />
              Tambah varian
            </Button>
          }
        />
        <DataTable
          columns={columns}
          rows={variants}
          rowKey={(row) => row.id}
          empty="Produk belum punya varian."
        />
        {!canSetPrice && (
          <p className="border-t border-rule px-4 py-2 text-xs text-muted">
            Harga hanya bisa diubah oleh pengguna dengan izin <span className="num">products.update_price</span>.
          </p>
        )}
      </Card>

      <Card>
        <CardHeader
          title="Riwayat harga"
          action={
            <div className="w-56">
              <Select
                aria-label="Pilih varian"
                value={historyVariant}
                onValueChange={setHistoryVariant}
                options={variants.map((variant) => ({ value: variant.id, label: variant.name }))}
              />
            </div>
          }
        />
        <ul className="divide-y divide-rule">
          {(history.data ?? []).map((row, index) => (
            <li
              key={`${row.changed_at}-${index}`}
              className="flex flex-wrap items-center justify-between gap-2 px-5 py-3 text-sm"
            >
              <span className="text-muted">
                {new Date(row.changed_at).toLocaleString('id-ID', {
                  dateStyle: 'medium',
                  timeStyle: 'short',
                })}
              </span>
              <span className="num">
                {row.old_price ? `${formatMoney(row.old_price)} → ` : ''}
                {formatMoney(row.new_price)}
              </span>
            </li>
          ))}
          {!history.isLoading && (history.data ?? []).length === 0 && (
            <li className="px-5 py-6 text-center text-sm text-muted">
              Belum ada perubahan harga tercatat.
            </li>
          )}
        </ul>
      </Card>

      <Dialog
        open={draft !== null}
        onOpenChange={(open) => !save.isPending && !open && setDraft(null)}
        title={draft?.id ? 'Ubah varian' : 'Varian baru'}
        description="Harga disimpan sebagai angka dari server; kasir tidak menghitung sendiri."
        footer={
          <>
            <Button variant="secondary" onClick={() => setDraft(null)} disabled={save.isPending}>
              Batal
            </Button>
            <Button
              onClick={() => draft && save.mutate(draft)}
              loading={save.isPending}
              disabled={!draft?.name.trim()}
            >
              Simpan
            </Button>
          </>
        }
      >
        {draft && (
          <div className="space-y-4">
            <Input
              label="Nama varian"
              placeholder="Contoh: Jumbo"
              value={draft.name}
              autoFocus
              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
            />
            <Input
              label="Akhiran SKU"
              placeholder="Contoh: JMB"
              value={draft.sku}
              onChange={(event) => setDraft({ ...draft, sku: event.target.value })}
            />
            <NumberInput
              label="Harga jual"
              value={draft.price}
              onValueChange={(price) => setDraft({ ...draft, price })}
              disabled={!canSetPrice}
              helper={canSetPrice ? undefined : 'Anda tidak punya izin mengubah harga.'}
            />
            <div className="flex flex-wrap gap-5">
              <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm">
                <input
                  type="checkbox"
                  className="size-5 accent-brand-600"
                  checked={draft.is_default}
                  onChange={(event) => setDraft({ ...draft, is_default: event.target.checked })}
                />
                Varian default
              </label>
              <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm">
                <input
                  type="checkbox"
                  className="size-5 accent-brand-600"
                  checked={draft.is_active}
                  onChange={(event) => setDraft({ ...draft, is_active: event.target.checked })}
                />
                Aktif dijual
              </label>
            </div>
            {error && (
              <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
                {error}
              </p>
            )}
          </div>
        )}
      </Dialog>
    </div>
  )
}

function wholeRupiah(amount: string): string {
  return amount.split('.')[0] ?? '0'
}

function toInput(variant: Variant, isDefault: boolean): VariantInput {
  return {
    name: variant.name,
    sku: variant.sku ?? '',
    price: variant.price,
    is_default: isDefault,
    is_active: variant.is_active,
    sort_order: variant.sort_order,
  }
}
