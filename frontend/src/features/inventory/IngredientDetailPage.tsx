import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useEffect } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { Controller } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { z } from 'zod'
import { inventoryApi } from '../../api/inventory'
import type { IngredientInput } from '../../api/inventory'
import { PageHeader } from '../../components/shared/PageHeader'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

const schema = z.object({
  code: z.string().trim().min(1, 'Kode bahan wajib diisi.'),
  name: z.string().trim().min(1, 'Nama bahan wajib diisi.'),
  category_id: z.string(),
  base_unit_id: z.string().min(1, 'Pilih satuan dasar.'),
  default_purchase_unit_id: z.string(),
  min_stock: z
    .string()
    .trim()
    .regex(/^\d+(\.\d+)?$/, 'Stok minimum harus angka, contoh 2.5.'),
  is_stock_product: z.boolean(),
})

type FormValues = z.infer<typeof schema>

const emptyValues: FormValues = {
  code: '',
  name: '',
  category_id: '',
  base_unit_id: '',
  default_purchase_unit_id: '',
  min_stock: '0',
  is_stock_product: false,
}

export function IngredientDetailPage() {
  const { id = 'new' } = useParams()
  const isNew = id === 'new'
  const canManage = useAuthStore((state) => state.hasPermission('ingredients.update'))

  const ingredient = useQuery({
    queryKey: ['ingredient', id],
    queryFn: async () => (await inventoryApi.ingredients.get(id)).data.data,
    enabled: !isNew,
  })

  return (
    <>
      <Link
        to="/ingredients"
        className="inline-flex min-h-11 items-center gap-1 text-sm text-muted hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Semua bahan
      </Link>
      <div className="mt-2">
        <PageHeader
          title={isNew ? 'Bahan baru' : (ingredient.data?.name ?? 'Detail bahan')}
          description={
            isNew
              ? 'Bahan dipakai oleh resep dan mengurangi stok saat pesanan dikirim ke dapur.'
              : `${ingredient.data?.code ?? ''} · ${ingredient.data?.category_name ?? 'Tanpa kategori'}`
          }
        />
      </div>

      {isNew ? (
        <IngredientForm />
      ) : ingredient.isLoading ? (
        <Skeleton className="mt-6 h-72 w-full" />
      ) : ingredient.isError ? (
        <Card className="mt-6 p-6 text-sm text-negative">{apiMessage(ingredient.error)}</Card>
      ) : ingredient.data ? (
        <div className="mt-6 space-y-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Stat label="Stok saat ini" value={quantity(ingredient.data.quantity_on_hand)} unit={ingredient.data.base_unit_code} />
            <Stat label="Stok minimum" value={quantity(ingredient.data.min_stock)} unit={ingredient.data.base_unit_code} />
            <Stat
              label="Biaya rata-rata"
              value={formatMoney(ingredient.data.avg_unit_cost)}
              unit={`/ ${ingredient.data.base_unit_code}`}
            />
          </div>
          <IngredientForm ingredient={ingredient.data} readOnlyStock />
          {canManage && <Movements ingredientId={ingredient.data.id} unit={ingredient.data.base_unit_code} />}
        </div>
      ) : null}
    </>
  )
}

function Stat({ label, value, unit }: { label: string; value: string; unit: string }) {
  return (
    <Card className="p-4">
      <p className="text-xs text-muted">{label}</p>
      <p className="num mt-1 text-xl text-ink">
        {value} <span className="text-sm text-muted">{unit}</span>
      </p>
    </Card>
  )
}

function IngredientForm({
  ingredient,
  readOnlyStock = false,
}: {
  ingredient?: import('../../api/inventory').Ingredient
  readOnlyStock?: boolean
}) {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canManage = useAuthStore((state) => state.hasPermission('ingredients.update'))
  const canCreate = useAuthStore((state) => state.hasPermission('ingredients.create'))
  const allowed = ingredient ? canManage : canCreate

  const units = useQuery({
    queryKey: ['units'],
    queryFn: async () => (await inventoryApi.units()).data.data,
    staleTime: 1000 * 60 * 60,
  })
  const categories = useQuery({
    queryKey: ['ingredient-categories'],
    queryFn: async () => (await inventoryApi.categories.list()).data.data,
  })

  const {
    register,
    handleSubmit,
    control,
    setValue,
    setError,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: ingredient
      ? {
          code: ingredient.code,
          name: ingredient.name,
          category_id: ingredient.category_id ?? '',
          base_unit_id: String(ingredient.base_unit_id),
          default_purchase_unit_id: ingredient.default_purchase_unit_id
            ? String(ingredient.default_purchase_unit_id)
            : '',
          min_stock: editableQuantity(ingredient.min_stock),
          is_stock_product: ingredient.is_stock_product,
        }
      : emptyValues,
  })

  const baseUnitId = useWatch({ control, name: 'base_unit_id' })

  // Satuan dasar pertama dipilih otomatis setelah daftar satuan termuat.
  useEffect(() => {
    if (!ingredient && units.data?.length) {
      setValue('base_unit_id', String(units.data[0].id))
    }
  }, [ingredient, setValue, units.data])

  const save = useMutation({
    mutationFn: (values: FormValues) => {
      const body: IngredientInput = {
        code: values.code.trim(),
        name: values.name.trim(),
        category_id: values.category_id || null,
        base_unit_id: Number(values.base_unit_id),
        default_purchase_unit_id: values.default_purchase_unit_id
          ? Number(values.default_purchase_unit_id)
          : null,
        min_stock: values.min_stock.trim(),
        is_stock_product: values.is_stock_product,
      }
      return ingredient
        ? inventoryApi.ingredients.update(ingredient.id, body)
        : inventoryApi.ingredients.create(body)
    },
    onSuccess: async (response) => {
      await queryClient.invalidateQueries({ queryKey: ['ingredients'] })
      await queryClient.invalidateQueries({ queryKey: ['ingredient', response.data.data.id] })
      toast.push(ingredient ? 'Bahan diperbarui.' : 'Bahan dibuat.')
      if (!ingredient) navigate(`/ingredients/${response.data.data.id}`, { replace: true })
    },
    onError: (error) => setError('root', { message: apiMessage(error) }),
  })

  const unitOptions = (units.data ?? []).map((unit) => ({
    value: String(unit.id),
    label: `${unit.name} (${unit.code})`,
  }))

  return (
    <form
      className="mt-4"
      onSubmit={handleSubmit((values) => save.mutate(values))}
      noValidate
    >
      <Card>
        <CardHeader
          title="Informasi bahan"
          action={
            readOnlyStock && ingredient && (
              <p className="text-xs text-muted">
                Stok tidak bisa diketik langsung. Pakai penyesuaian stok atau penerimaan barang.
              </p>
            )
          }
        />
        <div className="grid grid-cols-1 gap-5 p-5 md:grid-cols-2">
          <Input
            label="Kode bahan"
            placeholder="Contoh: BRS-01"
            error={errors.code?.message}
            disabled={!allowed}
            {...register('code')}
          />
          <Input
            label="Nama bahan"
            placeholder="Contoh: Beras pandan wangi"
            error={errors.name?.message}
            disabled={!allowed}
            {...register('name')}
          />

          <Controller
            control={control}
            name="category_id"
            render={({ field }) => (
              <Select
                label="Kategori"
                value={field.value || 'none'}
                onValueChange={(value) => field.onChange(value === 'none' ? '' : value)}
                disabled={!allowed}
                options={[
                  { value: 'none', label: 'Tanpa kategori' },
                  ...(categories.data ?? []).map((category) => ({
                    value: category.id,
                    label: category.name,
                  })),
                ]}
              />
            )}
          />

          <Controller
            control={control}
            name="base_unit_id"
            render={({ field }) => (
              <div>
                <Select
                  label="Satuan dasar"
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!allowed || readOnlyStock}
                  options={unitOptions}
                />
                {errors.base_unit_id && (
                  <p className="mt-1.5 text-sm text-negative">{errors.base_unit_id.message}</p>
                )}
              </div>
            )}
          />

          <Controller
            control={control}
            name="default_purchase_unit_id"
            render={({ field }) => (
              <Select
                label="Satuan beli"
                value={field.value || 'none'}
                onValueChange={(value) => field.onChange(value === 'none' ? '' : value)}
                disabled={!allowed}
                options={[{ value: 'none', label: 'Sama dengan satuan dasar' }, ...unitOptions]}
              />
            )}
          />

          <Input
            label={`Stok minimum (${unitLabel(units.data, baseUnitId)})`}
            inputMode="decimal"
            error={errors.min_stock?.message}
            disabled={!allowed}
            {...register('min_stock')}
          />

          <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-ink md:col-span-2">
            <input
              type="checkbox"
              className="size-5 accent-brand-600"
              disabled={!allowed}
              {...register('is_stock_product')}
            />
            Jadikan bahan ini produk stok (bisa dijual langsung tanpa resep)
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
        {allowed && (
          <div className="flex justify-end gap-2 border-t border-rule px-5 py-3">
            <Link to="/ingredients">
              <Button variant="secondary" type="button">
                Batal
              </Button>
            </Link>
            <Button type="submit" loading={save.isPending}>
              {ingredient ? 'Simpan perubahan' : 'Buat bahan'}
            </Button>
          </div>
        )}
      </Card>
    </form>
  )
}

function Movements({ ingredientId, unit }: { ingredientId: string; unit: string }) {
  const movements = useQuery({
    queryKey: ['ingredient-movements', ingredientId],
    queryFn: async () => (await inventoryApi.ingredients.movements(ingredientId)).data.data,
  })

  const columns: Column<import('../../api/inventory').StockMovement>[] = [
    {
      key: 'type',
      header: 'Jenis',
      render: (row) => <span className="text-sm">{movementLabel(row.movement_type)}</span>,
    },
    {
      key: 'delta',
      header: 'Perubahan',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className={Number(row.quantity_delta) < 0 ? 'num text-negative' : 'num text-positive'}>
          {Number(row.quantity_delta) > 0 ? '+' : ''}
          {quantity(row.quantity_delta)} {unit}
        </span>
      ),
    },
    {
      key: 'balance',
      header: 'Saldo',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num">
          {quantity(row.balance_after)} {unit}
        </span>
      ),
    },
    {
      key: 'when',
      header: 'Waktu',
      render: (row) => (
        <span className="text-sm text-muted">
          {new Date(row.created_at).toLocaleString('id-ID', {
            dateStyle: 'medium',
            timeStyle: 'short',
          })}
        </span>
      ),
    },
    {
      key: 'reason',
      header: 'Catatan',
      render: (row) => <span className="text-sm text-muted">{row.reason ?? row.reference_type}</span>,
    },
  ]

  return (
    <Card>
      <CardHeader title="Riwayat pergerakan stok" />
      <DataTable
        columns={columns}
        rows={movements.data ?? []}
        rowKey={(row) => row.id}
        loading={movements.isLoading}
        error={movements.isError ? apiMessage(movements.error) : undefined}
        onRetry={() => movements.refetch()}
        empty="Belum ada pergerakan stok."
      />
    </Card>
  )
}

const MOVEMENT_LABELS: Record<string, string> = {
  INITIAL_STOCK: 'Stok awal',
  PURCHASE: 'Pembelian',
  SALE: 'Penjualan',
  SALE_REVERSAL: 'Pembatalan penjualan',
  WASTE: 'Waste',
  WASTE_REVERSAL: 'Pembatalan waste',
  ADJUSTMENT: 'Penyesuaian',
  RETURN: 'Retur',
  TRANSFER_OUT: 'Transfer keluar',
  TRANSFER_IN: 'Transfer masuk',
}

function movementLabel(type: string): string {
  return MOVEMENT_LABELS[type] ?? type
}

function unitLabel(
  units: import('../../api/inventory').Unit[] | undefined,
  unitID: string,
): string {
  return units?.find((unit) => String(unit.id) === unitID)?.code ?? 'satuan'
}

function quantity(value: string): string {
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(Number(value))
}

// API mengirim NUMERIC dengan 4 desimal; buang nol ekor agar enak diedit.
function editableQuantity(value: string): string {
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}
