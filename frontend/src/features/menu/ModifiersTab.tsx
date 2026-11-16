import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { menuApi } from '../../api/menu'
import type { ModifierGroup, ModifierGroupInput, Product } from '../../api/menu'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'

interface OptionDraft {
  id?: string
  name: string
  price_delta: string
  is_default: boolean
  sort_order: number
}

interface GroupDraft {
  id?: string
  name: string
  min_select: number
  max_select: number
  is_required: boolean
  is_active: boolean
  sort_order: number
  options: OptionDraft[]
}

const newGroup: GroupDraft = {
  name: '',
  min_select: 0,
  max_select: 1,
  is_required: false,
  is_active: true,
  sort_order: 0,
  options: [{ name: '', price_delta: '0', is_default: false, sort_order: 0 }],
}

export function ModifiersTab({ product, onChanged }: { product: Product; onChanged: () => void }) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const canAssign = useAuthStore((state) => state.hasPermission('products.update'))
  const canManageGroups = useAuthStore((state) => state.hasPermission('modifiers.manage'))

  const [selected, setSelected] = useState<string[]>(product.modifier_group_ids ?? [])
  const [draft, setDraft] = useState<GroupDraft | null>(null)
  const [error, setError] = useState<string | null>(null)

  const groups = useQuery({
    queryKey: ['modifier-groups'],
    queryFn: async () => (await menuApi.modifiers.list()).data.data,
  })

  const saveAssignment = useMutation({
    mutationFn: () => menuApi.modifiers.setProductGroups(product.id, selected),
    onSuccess: () => {
      onChanged()
      toast.push('Modifier produk diperbarui.')
    },
    onError: (err) => toast.push(apiMessage(err), 'error'),
  })

  const saveGroup = useMutation({
    mutationFn: (values: GroupDraft) => {
      const body: ModifierGroupInput = {
        name: values.name.trim(),
        min_select: values.min_select,
        max_select: values.max_select,
        is_required: values.is_required,
        sort_order: values.sort_order,
        is_active: values.is_active,
        options: values.options.map((option, index) => ({
          id: option.id,
          name: option.name.trim(),
          price_delta: option.price_delta,
          is_default: option.is_default,
          sort_order: index,
        })),
      }
      return values.id
        ? menuApi.modifiers.update(values.id, body)
        : menuApi.modifiers.create(body)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['modifier-groups'] })
      toast.push(draft?.id ? 'Grup modifier diperbarui.' : 'Grup modifier dibuat.')
      setDraft(null)
    },
    onError: (err) => setError(apiMessage(err)),
  })

  function openGroup(group: ModifierGroup) {
    setError(null)
    setDraft({
      id: group.id,
      name: group.name,
      min_select: group.min_select,
      max_select: group.max_select,
      is_required: group.is_required,
      is_active: group.is_active,
      sort_order: group.sort_order,
      options: group.options
        .filter((option) => option.is_active)
        .map((option) => ({
          id: option.id,
          name: option.name,
          price_delta: wholeRupiah(option.price_delta),
          is_default: option.is_default,
          sort_order: option.sort_order,
        })),
    })
  }

  const assignedGroups = (groups.data ?? []).filter((group) => selected.includes(group.id))

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader
          title="Modifier produk"
          action={
            canManageGroups && (
              <Button size="sm" variant="secondary" onClick={() => setDraft({ ...newGroup })}>
                <Plus className="size-4" strokeWidth={2} />
                Grup baru
              </Button>
            )
          }
        />
        <div className="p-5">
          <p className="text-sm text-muted">
            Centang grup yang berlaku untuk menu ini. Contoh: level pedas, tambahan topping, pilihan
            es.
          </p>
          <div className="mt-4 grid grid-cols-1 gap-2 md:grid-cols-2">
            {(groups.data ?? []).map((group) => (
              <label
                key={group.id}
                className={cn(
                  'flex min-h-14 cursor-pointer items-start gap-3 rounded-control border px-3 py-2.5',
                  selected.includes(group.id) ? 'border-ink bg-raised' : 'border-line',
                )}
              >
                <input
                  type="checkbox"
                  className="mt-0.5 size-5 accent-brand-600"
                  checked={selected.includes(group.id)}
                  disabled={!canAssign}
                  onChange={(event) =>
                    setSelected((current) =>
                      event.target.checked
                        ? [...current, group.id]
                        : current.filter((id) => id !== group.id),
                    )
                  }
                />
                <span className="min-w-0 flex-1">
                  <span className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium text-ink">{group.name}</span>
                    {group.is_required && <Badge tone="yellow">Wajib</Badge>}
                    {!group.is_active && <Badge tone="gray">Nonaktif</Badge>}
                  </span>
                  <span className="mt-0.5 block text-xs text-muted">
                    Pilih {group.min_select}–{group.max_select} · {group.options.length} opsi
                  </span>
                </span>
                {canManageGroups && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(event) => {
                      event.preventDefault()
                      openGroup(group)
                    }}
                    aria-label={`Ubah grup ${group.name}`}
                  >
                    <Pencil className="size-4" strokeWidth={1.5} />
                  </Button>
                )}
              </label>
            ))}
          </div>
          {(groups.data ?? []).length === 0 && (
            <p className="py-6 text-center text-sm text-muted">
              Belum ada grup modifier. Buat grup pertama.
            </p>
          )}
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-rule px-5 py-3">
          <p className="text-xs text-muted">
            {assignedGroups.length} grup dipakai: {assignedGroups.map((group) => group.name).join(', ') || '—'}
          </p>
          <Button onClick={() => saveAssignment.mutate()} loading={saveAssignment.isPending} disabled={!canAssign}>
            Simpan modifier
          </Button>
        </div>
      </Card>

      <Dialog
        open={draft !== null}
        onOpenChange={(open) => !saveGroup.isPending && !open && setDraft(null)}
        title={draft?.id ? 'Ubah grup modifier' : 'Grup modifier baru'}
        description="Atur berapa opsi minimal dan maksimal yang harus dipilih kasir."
        footer={
          <>
            <Button variant="secondary" onClick={() => setDraft(null)} disabled={saveGroup.isPending}>
              Batal
            </Button>
            <Button
              onClick={() => draft && saveGroup.mutate(draft)}
              loading={saveGroup.isPending}
              disabled={!draft?.name.trim() || !draft.options.every((option) => option.name.trim())}
            >
              Simpan grup
            </Button>
          </>
        }
      >
        {draft && (
          <div className="space-y-4">
            <Input
              label="Nama grup"
              placeholder="Contoh: Level pedas"
              autoFocus
              value={draft.name}
              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
            />
            <div className="grid grid-cols-2 gap-3">
              <Input
                label="Minimal pilih"
                type="number"
                min={0}
                value={draft.min_select}
                onChange={(event) =>
                  setDraft({ ...draft, min_select: Math.max(0, Number(event.target.value) || 0) })
                }
              />
              <Input
                label="Maksimal pilih"
                type="number"
                min={0}
                value={draft.max_select}
                onChange={(event) =>
                  setDraft({ ...draft, max_select: Math.max(0, Number(event.target.value) || 0) })
                }
              />
            </div>
            <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm">
              <input
                type="checkbox"
                className="size-5 accent-brand-600"
                checked={draft.is_required}
                onChange={(event) => setDraft({ ...draft, is_required: event.target.checked })}
              />
              Wajib dipilih sebelum item masuk keranjang
            </label>

            <div>
              <p className="mb-2 text-sm font-medium text-ink">Opsi</p>
              <div className="space-y-2">
                {draft.options.map((option, index) => (
                  <div key={index} className="rounded-control border border-line p-3">
                    <div className="flex items-start gap-2">
                      <div className="min-w-0 flex-1">
                        <Input
                          aria-label={`Nama opsi ${index + 1}`}
                          placeholder="Contoh: Pedas sedang"
                          value={option.name}
                          onChange={(event) =>
                            setDraft({
                              ...draft,
                              options: draft.options.map((item, itemIndex) =>
                                itemIndex === index ? { ...item, name: event.target.value } : item,
                              ),
                            })
                          }
                        />
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-negative"
                        disabled={draft.options.length === 1}
                        onClick={() =>
                          setDraft({
                            ...draft,
                            options: draft.options.filter((_, itemIndex) => itemIndex !== index),
                          })
                        }
                        aria-label={`Hapus opsi ${index + 1}`}
                      >
                        <Trash2 className="size-4" strokeWidth={1.5} />
                      </Button>
                    </div>
                    <div className="mt-2 flex flex-wrap items-end gap-3">
                      <div className="w-40">
                        <NumberInput
                          label="Tambahan harga"
                          value={option.price_delta}
                          onValueChange={(value) =>
                            setDraft({
                              ...draft,
                              options: draft.options.map((item, itemIndex) =>
                                itemIndex === index ? { ...item, price_delta: value } : item,
                              ),
                            })
                          }
                        />
                      </div>
                      <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm">
                        <input
                          type="checkbox"
                          className="size-5 accent-brand-600"
                          checked={option.is_default}
                          onChange={(event) =>
                            setDraft({
                              ...draft,
                              options: draft.options.map((item, itemIndex) =>
                                itemIndex === index
                                  ? { ...item, is_default: event.target.checked }
                                  : item,
                              ),
                            })
                          }
                        />
                        Default
                      </label>
                    </div>
                  </div>
                ))}
              </div>
              <Button
                variant="secondary"
                size="sm"
                className="mt-2"
                onClick={() =>
                  setDraft({
                    ...draft,
                    options: [
                      ...draft.options,
                      { name: '', price_delta: '0', is_default: false, sort_order: draft.options.length },
                    ],
                  })
                }
              >
                <Plus className="size-4" strokeWidth={2} />
                Tambah opsi
              </Button>
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
