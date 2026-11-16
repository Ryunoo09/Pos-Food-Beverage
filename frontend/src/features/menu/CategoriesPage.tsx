import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { menuApi } from '../../api/menu'
import type { Category } from '../../api/menu'
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

const schema = z.object({
  name: z.string().trim().min(1, 'Nama kategori wajib diisi.').max(80, 'Maksimal 80 karakter.'),
  sort_order: z.number().int('Urutan harus bilangan bulat.').min(0, 'Urutan tidak boleh negatif.'),
  is_active: z.boolean(),
})

type FormValues = z.infer<typeof schema>

const emptyValues: FormValues = { name: '', sort_order: 0, is_active: true }

export function CategoriesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canManage = useAuthStore((state) => state.hasPermission('categories.manage'))

  const [editing, setEditing] = useState<Category | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [deleting, setDeleting] = useState<Category | null>(null)

  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: async () => (await menuApi.categories.list()).data.data,
  })

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
    setError,
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: emptyValues })

  function openCreate() {
    setEditing(null)
    reset(emptyValues)
    setFormOpen(true)
  }

  function openEdit(category: Category) {
    setEditing(category)
    reset({
      name: category.name,
      sort_order: category.sort_order,
      is_active: category.is_active,
    })
    setFormOpen(true)
  }

  const save = useMutation({
    mutationFn: (values: FormValues) =>
      editing
        ? menuApi.categories.update(editing.id, values)
        : menuApi.categories.create(values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['categories'] })
      toast.push(editing ? 'Kategori diperbarui.' : 'Kategori dibuat.')
      setFormOpen(false)
    },
    onError: (error) => setError('root', { message: apiMessage(error) }),
  })

  const remove = useMutation({
    mutationFn: (id: string) => menuApi.categories.remove(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['categories'] })
      toast.push('Kategori dihapus.')
      setDeleting(null)
    },
    onError: (error) => {
      toast.push(apiMessage(error), 'error')
      setDeleting(null)
    },
  })

  const columns: Column<Category>[] = [
    {
      key: 'name',
      header: 'Kategori',
      render: (row) => (
        <div>
          <p className="font-medium text-ink">{row.name}</p>
          <p className="text-xs text-muted">Urutan {row.sort_order}</p>
        </div>
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
      className: 'w-28 text-right',
      render: (row) =>
        canManage ? (
          <div className="flex justify-end gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => openEdit(row)}
              aria-label={`Ubah ${row.name}`}
            >
              <Pencil className="size-4" strokeWidth={1.5} />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="text-negative"
              onClick={() => setDeleting(row)}
              aria-label={`Hapus ${row.name}`}
            >
              <Trash2 className="size-4" strokeWidth={1.5} />
            </Button>
          </div>
        ) : null,
    },
  ]

  return (
    <>
      <PageHeader
        title="Kategori menu"
        description="Kelompokkan menu agar kasir dan pelanggan cepat menemukan hidangan."
        action={
          canManage && (
            <Button onClick={openCreate}>
              <Plus className="size-4" strokeWidth={2} />
              Tambah kategori
            </Button>
          )
        }
      />

      <Card className="mt-6">
        <DataTable
          columns={columns}
          rows={categories.data ?? []}
          rowKey={(row) => row.id}
          loading={categories.isLoading}
          error={categories.isError ? apiMessage(categories.error) : undefined}
          onRetry={() => categories.refetch()}
          empty="Belum ada kategori. Tambahkan kategori pertama."
        />
      </Card>

      <Dialog
        open={formOpen}
        onOpenChange={(open) => !save.isPending && setFormOpen(open)}
        title={editing ? 'Ubah kategori' : 'Kategori baru'}
        description="Nama kategori tampil di layar kasir dan menu pelanggan."
        footer={
          <>
            <Button variant="secondary" onClick={() => setFormOpen(false)} disabled={save.isPending}>
              Batal
            </Button>
            <Button
              onClick={handleSubmit((values) => save.mutate(values))}
              loading={save.isPending || isSubmitting}
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
            label="Nama kategori"
            placeholder="Contoh: Makanan berat"
            autoFocus
            error={errors.name?.message}
            {...register('name')}
          />
          <Input
            label="Urutan tampil"
            type="number"
            min={0}
            error={errors.sort_order?.message}
            helper="Angka kecil tampil lebih dulu."
            {...register('sort_order', { valueAsNumber: true })}
          />
          <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-ink">
            <input
              type="checkbox"
              className="size-5 accent-brand-600"
              {...register('is_active')}
            />
            Tampilkan kategori ini
          </label>
          {errors.root && (
            <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
              {errors.root.message}
            </p>
          )}
        </form>
      </Dialog>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus kategori?"
        description={`Kategori "${deleting?.name ?? ''}" akan dihapus. Kategori yang masih dipakai produk tidak bisa dihapus.`}
        confirmLabel="Hapus"
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onCancel={() => setDeleting(null)}
      />
    </>
  )
}
