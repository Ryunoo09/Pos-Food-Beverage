import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { inventoryApi } from '../../api/inventory'
import { supplierApi } from '../../api/suppliers'
import type { Supplier, SupplierInput } from '../../api/suppliers'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

const schema = z.object({
  name: z.string().trim().min(1, 'Nama supplier wajib diisi.'),
  contact_person: z.string(),
  phone: z.string(),
  email: z.union([z.literal(''), z.string().email('Format email tidak valid.')]),
  address: z.string(),
  note: z.string(),
})

type FormValues = z.infer<typeof schema>

const emptyValues: FormValues = {
  name: '',
  contact_person: '',
  phone: '',
  email: '',
  address: '',
  note: '',
}

export function SuppliersPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canManage = useAuthStore((state) => state.hasPermission('suppliers.update'))
  const canCreate = useAuthStore((state) => state.hasPermission('suppliers.create'))

  const [search, setSearch] = useState('')
  const [q, setQ] = useState('')
  const [editing, setEditing] = useState<Supplier | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [viewing, setViewing] = useState<Supplier | null>(null)
  const [deactivating, setDeactivating] = useState<Supplier | null>(null)
  const [selectedIngredients, setSelectedIngredients] = useState<string[]>([])

  const suppliers = useQuery({
    queryKey: ['suppliers', q],
    queryFn: async () => (await supplierApi.list({ q: q || undefined })).data.data,
  })
  const ingredients = useQuery({
    queryKey: ['ingredients', 'options'],
    queryFn: async () =>
      (await inventoryApi.ingredients.list({ per_page: 100, is_active: true })).data.data,
  })

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: emptyValues })

  function openCreate() {
    setEditing(null)
    setSelectedIngredients([])
    reset(emptyValues)
    setFormOpen(true)
  }

  function openEdit(supplier: Supplier) {
    setEditing(supplier)
    setSelectedIngredients((supplier.ingredients ?? []).map((item) => item.ingredient_id))
    reset({
      name: supplier.name,
      contact_person: supplier.contact_person ?? '',
      phone: supplier.phone ?? '',
      email: supplier.email ?? '',
      address: supplier.address ?? '',
      note: supplier.note ?? '',
    })
    setFormOpen(true)
  }

  const save = useMutation({
    mutationFn: async (values: FormValues) => {
      const body: SupplierInput = {
        name: values.name.trim(),
        contact_person: values.contact_person.trim() || null,
        phone: values.phone.trim() || null,
        email: values.email.trim() || null,
        address: values.address.trim() || null,
        note: values.note.trim() || null,
      }
      const supplier = editing
        ? (await supplierApi.update(editing.id, body)).data.data
        : (await supplierApi.create(body)).data.data
      await supplierApi.setIngredients(
        supplier.id,
        selectedIngredients.map((ingredientID) => ({
          ingredient_id: ingredientID,
          purchase_unit_id: null,
          last_price: null,
          lead_time_days: null,
          is_preferred: false,
        })),
      )
      return supplier
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['suppliers'] })
      toast.push(editing ? 'Supplier diperbarui.' : 'Supplier dibuat.')
      setFormOpen(false)
    },
    onError: (error) => setError('root', { message: apiMessage(error) }),
  })

  const deactivate = useMutation({
    mutationFn: (id: string) => supplierApi.deactivate(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['suppliers'] })
      toast.push('Supplier dinonaktifkan.')
      setDeactivating(null)
    },
    onError: (error) => {
      toast.push(apiMessage(error), 'error')
      setDeactivating(null)
    },
  })

  const columns: Column<Supplier>[] = [
    {
      key: 'name',
      header: 'Supplier',
      render: (row) => (
        <button
          type="button"
          onClick={() => setViewing(row)}
          className="text-left font-medium text-ink hover:underline"
        >
          {row.name}
          <span className="block text-xs font-normal text-muted">
            {row.contact_person ?? 'Tanpa kontak'}
          </span>
        </button>
      ),
    },
    {
      key: 'phone',
      header: 'Telepon',
      render: (row) => <span className="num text-sm">{row.phone ?? '—'}</span>,
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
      className: 'w-40 text-right',
      render: (row) => (
        <div className="flex justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => setViewing(row)}>
            Detail
          </Button>
          {canManage && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => openEdit(row)}
              aria-label={`Ubah supplier ${row.name}`}
            >
              <Pencil className="size-4" strokeWidth={1.5} />
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Supplier"
        description="Pemasok bahan baku, daftar bahan, dan riwayat pembelian."
        action={
          canCreate && (
            <Button onClick={openCreate}>
              <Plus className="size-4" strokeWidth={2} />
              Tambah supplier
            </Button>
          )
        }
      />

      <form
        className="mt-6 max-w-md"
        onSubmit={(event) => {
          event.preventDefault()
          setQ(search.trim())
        }}
      >
        <Input
          aria-label="Cari supplier"
          placeholder="Cari nama supplier…"
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

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={suppliers.data ?? []}
          rowKey={(row) => row.id}
          loading={suppliers.isLoading}
          error={suppliers.isError ? apiMessage(suppliers.error) : undefined}
          onRetry={() => suppliers.refetch()}
          empty="Belum ada supplier."
        />
      </Card>

      <Dialog
        open={formOpen}
        onOpenChange={(open) => !save.isPending && setFormOpen(open)}
        title={editing ? 'Ubah supplier' : 'Supplier baru'}
        description="Data kontak dipakai saat membuat pesanan pembelian."
        footer={
          <>
            <Button variant="secondary" onClick={() => setFormOpen(false)} disabled={save.isPending}>
              Batal
            </Button>
            <Button
              onClick={handleSubmit((values) => save.mutate(values))}
              loading={save.isPending}
            >
              Simpan
            </Button>
          </>
        }
      >
        <form
          className="space-y-4"
          onSubmit={handleSubmit((values) => save.mutate(values))}
          noValidate
        >
          <Input
            label="Nama supplier"
            placeholder="Contoh: CV Sumber Pangan"
            autoFocus
            error={errors.name?.message}
            {...register('name')}
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input label="Kontak" placeholder="Nama PIC" {...register('contact_person')} />
            <Input label="Telepon" inputMode="tel" {...register('phone')} />
          </div>
          <Input label="Email" type="email" error={errors.email?.message} {...register('email')} />
          <Input label="Alamat" {...register('address')} />
          <Input label="Catatan" {...register('note')} />
          <div>
            <p className="mb-2 text-sm font-medium text-ink">Bahan yang disuplai</p>
            <div className="max-h-56 space-y-1 overflow-y-auto rounded-control border border-line p-2">
              {(ingredients.data ?? []).map((ingredient) => (
                <label
                  key={ingredient.id}
                  className="flex min-h-11 cursor-pointer items-center gap-3 rounded-control px-2 text-sm hover:bg-raised"
                >
                  <input
                    type="checkbox"
                    className="size-5 accent-brand-600"
                    checked={selectedIngredients.includes(ingredient.id)}
                    onChange={(event) =>
                      setSelectedIngredients((current) =>
                        event.target.checked
                          ? [...current, ingredient.id]
                          : current.filter((id) => id !== ingredient.id),
                      )
                    }
                  />
                  {ingredient.name}
                  <span className="num ml-auto text-xs text-muted">{ingredient.code}</span>
                </label>
              ))}
            </div>
          </div>
          {errors.root && (
            <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
              {errors.root.message}
            </p>
          )}
        </form>
      </Dialog>

      <SupplierDetail
        supplier={viewing}
        canEdit={canManage}
        onClose={() => setViewing(null)}
        onEdit={(supplier) => {
          setViewing(null)
          openEdit(supplier)
        }}
        onDeactivate={(supplier) => {
          setViewing(null)
          setDeactivating(supplier)
        }}
      />

      <ConfirmDialog
        open={deactivating !== null}
        title="Nonaktifkan supplier?"
        description={`"${deactivating?.name ?? ''}" tidak bisa dipilih pada pembelian baru. Riwayat pembelian tetap tersimpan.`}
        confirmLabel="Nonaktifkan"
        loading={deactivate.isPending}
        onConfirm={() => deactivating && deactivate.mutate(deactivating.id)}
        onCancel={() => setDeactivating(null)}
      />
    </>
  )
}

function SupplierDetail({
  supplier,
  canEdit,
  onClose,
  onEdit,
  onDeactivate,
}: {
  supplier: Supplier | null
  canEdit: boolean
  onClose: () => void
  onEdit: (supplier: Supplier) => void
  onDeactivate: (supplier: Supplier) => void
}) {
  const history = useQuery({
    queryKey: ['supplier-purchases', supplier?.id],
    queryFn: async () => (await supplierApi.purchaseHistory(supplier!.id)).data.data,
    enabled: supplier !== null,
  })

  return (
    <Dialog
      open={supplier !== null}
      onOpenChange={(open) => !open && onClose()}
      title={supplier?.name ?? ''}
      description={supplier?.contact_person ?? 'Detail supplier'}
      footer={
        supplier && (
          <>
            {supplier.is_active && (
              <Button variant="ghost" className="mr-auto text-negative" onClick={() => onDeactivate(supplier)}>
                Nonaktifkan
              </Button>
            )}
            {canEdit && <Button onClick={() => onEdit(supplier)}>Ubah supplier</Button>}
          </>
        )
      }
    >
      {supplier && (
        <div className="space-y-5">
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <Detail label="Telepon" value={supplier.phone} />
            <Detail label="Email" value={supplier.email} />
            <Detail label="Alamat" value={supplier.address} className="col-span-2" />
          </dl>

          <div>
            <p className="mb-2 text-sm font-medium text-ink">Bahan disuplai</p>
            {(supplier.ingredients ?? []).length === 0 ? (
              <p className="text-sm text-muted">Belum ada bahan yang ditautkan.</p>
            ) : (
              <ul className="divide-y divide-rule rounded-control border border-line">
                {(supplier.ingredients ?? []).map((item) => (
                  <li key={item.ingredient_id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
                    <span>{item.ingredient_name ?? item.ingredient_id}</span>
                    {item.last_price && <span className="num text-muted">{formatMoney(item.last_price)}</span>}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div>
            <p className="mb-2 text-sm font-medium text-ink">Riwayat pembelian</p>
            {history.isLoading ? (
              <p className="text-sm text-muted">Memuat…</p>
            ) : (history.data ?? []).length === 0 ? (
              <p className="text-sm text-muted">Belum ada pembelian dari supplier ini.</p>
            ) : (
              <ul className="divide-y divide-rule rounded-control border border-line">
                {history.data?.map((row) => (
                  <li key={row.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
                    <span>
                      <span className="num">{row.code}</span>
                      <span className="block text-xs text-muted">
                        {row.order_date} · {row.item_count} item
                      </span>
                    </span>
                    <span className="num">{formatMoney(row.total_amount)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </Dialog>
  )
}

function Detail({ label, value, className }: { label: string; value?: string; className?: string }) {
  return (
    <div className={className}>
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="mt-0.5 text-ink">{value ?? '—'}</dd>
    </div>
  )
}
