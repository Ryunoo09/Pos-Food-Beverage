import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { z } from 'zod'
import { inventoryApi } from '../../api/inventory'
import { menuApi } from '../../api/menu'
import type { Product, ProductInput } from '../../api/menu'
import { PageHeader } from '../../components/shared/PageHeader'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { Select } from '../../components/ui/Select'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { ModifiersTab } from './ModifiersTab'
import { VariantsTab } from './VariantsTab'

const schema = z.object({
  name: z.string().trim().min(1, 'Nama menu wajib diisi.').max(80, 'Maksimal 80 karakter.'),
  sku: z.string().trim().min(1, 'SKU wajib diisi.'),
  category_id: z.string().min(1, 'Pilih kategori.'),
  description: z.string(),
  product_type: z.enum(['FOOD', 'BEVERAGE', 'OTHER']),
  stock_mode: z.enum(['TRACK_BY_RECIPE', 'TRACK_AS_STOCK_ITEM', 'NO_TRACKING']),
  stock_ingredient_id: z.string(),
  kitchen_station: z.enum(['', 'KITCHEN', 'BAR']),
  allow_zero_price: z.boolean(),
  sort_order: z.number().int().min(0),
  base_price: z.string(),
})

type FormValues = z.infer<typeof schema>

const emptyValues: FormValues = {
  name: '',
  sku: '',
  category_id: '',
  description: '',
  product_type: 'FOOD',
  stock_mode: 'TRACK_BY_RECIPE',
  stock_ingredient_id: '',
  kitchen_station: '',
  allow_zero_price: false,
  sort_order: 0,
  base_price: '0',
}

type Tab = 'form' | 'variants' | 'modifiers'

export function ProductDetailPage() {
  const { id = 'new' } = useParams()
  const isNew = id === 'new'
  const [tab, setTab] = useState<Tab>('form')

  const product = useQuery({
    queryKey: ['product', id],
    queryFn: async () => (await menuApi.products.get(id)).data.data,
    enabled: !isNew,
  })

  return (
    <>
      <Link
        to="/menu"
        className="inline-flex min-h-11 items-center gap-1 text-sm text-muted hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua menu
      </Link>

      <div className="mt-2">
        <PageHeader
          title={isNew ? 'Menu baru' : (product.data?.name ?? 'Detail menu')}
          description={
            isNew
              ? 'Buat menu, lalu atur varian dan modifier agar kasir bisa menjualnya.'
              : `${product.data?.sku ?? ''} · ${product.data?.category_name ?? ''}`
          }
        />
      </div>

      {isNew ? (
        <ProductForm />
      ) : product.isLoading ? (
        <div className="mt-6 space-y-3">
          <Skeleton className="h-10 w-64" />
          <Skeleton className="h-72 w-full" />
        </div>
      ) : product.isError ? (
        <Card className="mt-6 p-6 text-sm text-negative">{apiMessage(product.error)}</Card>
      ) : product.data ? (
        <>
          <nav className="mt-6 flex gap-1 border-b border-line" aria-label="Bagian produk">
            {[
              { key: 'form' as const, label: 'Info menu' },
              { key: 'variants' as const, label: 'Varian & harga' },
              { key: 'modifiers' as const, label: 'Modifier' },
            ].map((item) => (
              <button
                key={item.key}
                type="button"
                onClick={() => setTab(item.key)}
                aria-current={tab === item.key ? 'page' : undefined}
                className={cn(
                  '-mb-px min-h-11 border-b-2 px-4 text-sm font-medium',
                  tab === item.key
                    ? 'border-ink text-ink'
                    : 'border-transparent text-muted hover:text-ink',
                )}
              >
                {item.label}
              </button>
            ))}
          </nav>

          <div className="mt-6">
            {tab === 'form' && <ProductForm product={product.data} />}
            {tab === 'variants' && (
              <VariantsTab product={product.data} onChanged={() => product.refetch()} />
            )}
            {tab === 'modifiers' && (
              <ModifiersTab product={product.data} onChanged={() => product.refetch()} />
            )}
          </div>
        </>
      ) : null}
    </>
  )
}

function ProductForm({ product }: { product?: Product }) {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canSetPrice = useAuthStore((state) => state.hasPermission('products.update_price'))

  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: async () => (await menuApi.categories.list()).data.data,
  })
  const ingredients = useQuery({
    queryKey: ['ingredients', 'options'],
    queryFn: async () =>
      (await inventoryApi.ingredients.list({ per_page: 100, is_active: true })).data.data,
  })

  const {
    register,
    handleSubmit,
    control,
    setError,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: product
      ? {
          name: product.name,
          sku: product.sku,
          category_id: product.category_id,
          description: product.description ?? '',
          product_type: product.product_type as FormValues['product_type'],
          stock_mode: product.stock_mode as FormValues['stock_mode'],
          stock_ingredient_id: product.stock_ingredient_id ?? '',
          kitchen_station: (product.kitchen_station as FormValues['kitchen_station']) ?? '',
          allow_zero_price: product.allow_zero_price,
          sort_order: product.sort_order,
          base_price: '0',
        }
      : emptyValues,
  })

  const stockMode = useWatch({ control, name: 'stock_mode' })

  const save = useMutation({
    mutationFn: async (values: FormValues) => {
      const body: ProductInput = {
        category_id: values.category_id,
        sku: values.sku,
        name: values.name,
        description: values.description.trim() || null,
        product_type: values.product_type,
        stock_mode: values.stock_mode,
        stock_ingredient_id: values.stock_ingredient_id || null,
        kitchen_station: values.kitchen_station || null,
        allow_zero_price: values.allow_zero_price,
        sort_order: values.sort_order,
        variants: product
          ? []
          : [
              {
                name: 'Reguler',
                sku: '',
                price: values.base_price,
                is_default: true,
                sort_order: 0,
              },
            ],
      }
      return product
        ? menuApi.products.update(product.id, body)
        : menuApi.products.create(body)
    },
    onSuccess: async (response) => {
      await queryClient.invalidateQueries({ queryKey: ['products'] })
      await queryClient.invalidateQueries({ queryKey: ['product', response.data.data.id] })
      toast.push(product ? 'Menu diperbarui.' : 'Menu dibuat. Atur varian bila perlu.')
      if (!product) navigate(`/menu/${response.data.data.id}`, { replace: true })
    },
    onError: (error) => setError('root', { message: apiMessage(error) }),
  })

  return (
    <form
      id="product-form"
      className="mt-4"
      onSubmit={handleSubmit((values) => {
        if (!values.category_id) {
          setError('category_id', { message: 'Pilih kategori.' })
          return
        }
        if (values.stock_mode === 'TRACK_AS_STOCK_ITEM' && !values.stock_ingredient_id) {
          setError('stock_ingredient_id', { message: 'Pilih bahan stok untuk mode ini.' })
          return
        }
        save.mutate(values)
      })}
      noValidate
    >
      <Card>
        <div className="grid grid-cols-1 gap-5 p-5 md:grid-cols-2">
          <Input
            label="Nama menu"
            placeholder="Contoh: Nasi goreng kampung"
            error={errors.name?.message}
            {...register('name')}
          />
          <Input label="SKU" placeholder="Contoh: NGK-01" error={errors.sku?.message} {...register('sku')} />
          <Controller
            control={control}
            name="category_id"
            render={({ field }) => (
              <div>
                <Select
                  label="Kategori"
                  value={field.value}
                  onValueChange={field.onChange}
                  options={(categories.data ?? []).map((category) => ({
                    value: category.id,
                    label: category.name,
                  }))}
                />
                {errors.category_id && (
                  <p className="mt-1.5 text-sm text-negative">{errors.category_id.message}</p>
                )}
              </div>
            )}
          />
          <Controller
            control={control}
            name="product_type"
            render={({ field }) => (
              <Select
                label="Tipe"
                value={field.value}
                onValueChange={field.onChange}
                options={[
                  { value: 'FOOD', label: 'Makanan' },
                  { value: 'BEVERAGE', label: 'Minuman' },
                  { value: 'OTHER', label: 'Lainnya' },
                ]}
              />
            )}
          />
          <Controller
            control={control}
            name="stock_mode"
            render={({ field }) => (
              <Select
                label="Mode stok"
                value={field.value}
                onValueChange={field.onChange}
                options={[
                  { value: 'TRACK_BY_RECIPE', label: 'Lacak lewat resep' },
                  { value: 'TRACK_AS_STOCK_ITEM', label: 'Lacak sebagai bahan stok' },
                  { value: 'NO_TRACKING', label: 'Tanpa pelacakan stok' },
                ]}
              />
            )}
          />
          <Controller
            control={control}
            name="kitchen_station"
            render={({ field }) => (
              <Select
                label="Stasiun dapur"
                value={field.value || 'none'}
                onValueChange={(value) => field.onChange(value === 'none' ? '' : value)}
                options={[
                  { value: 'none', label: 'Ikut default' },
                  { value: 'KITCHEN', label: 'Dapur' },
                  { value: 'BAR', label: 'Bar' },
                ]}
              />
            )}
          />

          {stockMode === 'TRACK_AS_STOCK_ITEM' && (
            <Controller
              control={control}
              name="stock_ingredient_id"
              render={({ field }) => (
                <div>
                  <Select
                    label="Bahan stok"
                    value={field.value}
                    onValueChange={field.onChange}
                    options={(ingredients.data ?? []).map((ingredient) => ({
                      value: ingredient.id,
                      label: `${ingredient.name} (${ingredient.base_unit_code})`,
                    }))}
                  />
                  {errors.stock_ingredient_id && (
                    <p className="mt-1.5 text-sm text-negative">
                      {errors.stock_ingredient_id.message}
                    </p>
                  )}
                </div>
              )}
            />
          )}

          <div className="md:col-span-2">
            <label className="mb-1 block text-sm font-medium text-ink" htmlFor="description">
              Deskripsi
            </label>
            <textarea
              id="description"
              rows={3}
              placeholder="Bahan utama, porsi, atau catatan lain yang membantu kasir."
              className="w-full rounded-control border border-control bg-surface px-3 py-2 text-sm text-ink outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/20"
              {...register('description')}
            />
          </div>

          <Input
            label="Urutan tampil"
            type="number"
            min={0}
            error={errors.sort_order?.message}
            {...register('sort_order', { valueAsNumber: true })}
          />
          {!product && (
            <Controller
              control={control}
              name="base_price"
              render={({ field }) => (
                <NumberInput
                  label="Harga dasar"
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!canSetPrice}
                  helper={
                    canSetPrice
                      ? 'Dibuat sebagai varian "Reguler". Varian lain ditambahkan setelah menu disimpan.'
                      : 'Anda tidak punya izin mengubah harga. Minta manajer.'
                  }
                />
              )}
            />
          )}

          <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-ink">
            <input type="checkbox" className="size-5 accent-brand-600" {...register('allow_zero_price')} />
            Izinkan harga nol (mis. bonus atau tester)
          </label>

          {errors.root && (
            <p
              role="alert"
              className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink md:col-span-2"
            >
              {errors.root.message}
            </p>
          )}
        </div>
        <div className="flex justify-end gap-2 border-t border-rule px-5 py-3">
          <Link to="/menu">
            <Button variant="secondary" type="button">
              Batal
            </Button>
          </Link>
          <Button form="product-form" type="submit" loading={save.isPending}>
            {product ? 'Simpan perubahan' : 'Buat menu'}
          </Button>
        </div>
      </Card>
    </form>
  )
}
