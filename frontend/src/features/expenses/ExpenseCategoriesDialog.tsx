import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { expenseApi } from '../../api/expenses'
import type { ExpenseCategory } from '../../api/expenses'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'

/**
 * Kategori pengeluaran dipakai lintas outlet dalam satu usaha, jadi daftarnya
 * dikelola di dialog kecil, bukan halaman penuh. Kategori tidak dihapus —
 * hanya dinonaktifkan — supaya pengeluaran lama tetap punya label.
 */
export function ExpenseCategoriesDialog({
  open,
  onOpenChange,
  categories,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  categories: ExpenseCategory[]
}) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['expense-categories'] })

  const create = useMutation({
    mutationFn: (value: string) => expenseApi.categories.create({ name: value }),
    onSuccess: async () => {
      setName('')
      await invalidate()
      toast.push('Kategori ditambahkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const toggle = useMutation({
    mutationFn: ({ id, isActive }: { id: string; isActive: boolean }) =>
      expenseApi.categories.update(id, { is_active: isActive }),
    onSuccess: async () => {
      await invalidate()
      toast.push('Kategori diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !create.isPending && onOpenChange(next)}
      title="Kategori pengeluaran"
      description="Kategori nonaktif tidak muncul saat mencatat pengeluaran baru, tetapi riwayat lama tetap menyimpannya."
      footer={
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Tutup
        </Button>
      }
    >
      <form
        className="flex items-end gap-2"
        onSubmit={(event) => {
          event.preventDefault()
          if (name.trim()) create.mutate(name.trim())
        }}
      >
        <Input
          label="Kategori baru"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Listrik & air"
        />
        <Button type="submit" loading={create.isPending} disabled={name.trim() === ''}>
          Tambah
        </Button>
      </form>

      <ul className="mt-4 max-h-64 divide-y divide-rule overflow-y-auto">
        {categories.length === 0 && (
          <li className="py-6 text-center text-sm text-muted">Belum ada kategori.</li>
        )}
        {categories.map((category) => (
          <li key={category.id} className="flex min-h-12 items-center justify-between gap-3 py-2">
            <div className="min-w-0">
              <p className="truncate text-sm">{category.name}</p>
              {!category.is_active && (
                <span className="mt-1 inline-flex">
                  <Badge tone="gray">Nonaktif</Badge>
                </span>
              )}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => toggle.mutate({ id: category.id, isActive: !category.is_active })}
              disabled={toggle.isPending}
            >
              {category.is_active ? 'Nonaktifkan' : 'Aktifkan'}
            </Button>
          </li>
        ))}
      </ul>
    </Dialog>
  )
}
