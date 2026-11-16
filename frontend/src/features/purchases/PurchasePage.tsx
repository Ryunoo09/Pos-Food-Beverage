import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { inventoryApi } from '../../api/inventory'
import type { Unit } from '../../api/inventory'
import { PO_STATUS_LABELS, PO_STATUSES, purchaseApi } from '../../api/purchases'
import type { PurchaseOrder, PurchaseOrderItemInput } from '../../api/purchases'
import { supplierApi } from '../../api/suppliers'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { purchaseStatusTone } from '../../utils/status'

const perPage = 20

interface DraftLine extends PurchaseOrderItemInput {
  key: number
}

export function PurchasePage() {
  const toast = useToast()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('purchases.create')

  const [params, setParams] = useSearchParams()
  const status = params.get('status') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [createOpen, setCreateOpen] = useState(false)

  const list = useQuery({
    queryKey: ['purchase-orders', { status, page }],
    queryFn: async () =>
      (await purchaseApi.list({ status: status || undefined, page, per_page: perPage })).data,
  })

  const suppliers = useQuery({
    queryKey: ['suppliers', { active: true }],
    queryFn: async () => (await supplierApi.list({ is_active: true })).data.data,
    enabled: createOpen,
  })

  const ingredients = useQuery({
    queryKey: ['ingredients', { active: true, q: '' }],
    queryFn: async () =>
      (await inventoryApi.ingredients.list({ is_active: true, per_page: 100 })).data.data,
    enabled: createOpen,
  })

  // purchase_unit_id harus unit id asli; itu dipakai backend untuk konversi ke
  // satuan dasar.
  const units = useQuery({
    queryKey: ['units'],
    queryFn: async () => (await inventoryApi.units()).data.data,
    enabled: createOpen,
  })

  const [supplierID, setSupplierID] = useState('')
  const [expectedDate, setExpectedDate] = useState('')
  const [note, setNote] = useState('')
  const [discount, setDiscount] = useState('0')
  const [lines, setLines] = useState<DraftLine[]>([
    { key: 1, ingredient_id: '', purchase_unit_id: 0, quantity_ordered: '', unit_price: '', discount_amount: '0' },
  ])

  const create = useMutation({
    mutationFn: () =>
      purchaseApi.create({
        supplier_id: supplierID,
        expected_date: expectedDate || null,
        note: note.trim() || null,
        discount_amount: discount.trim() || '0',
        items: lines
          .filter((line) => line.ingredient_id !== '' && Number(line.quantity_ordered) > 0)
          .map(({ ingredient_id, purchase_unit_id, quantity_ordered, unit_price, discount_amount }) => ({
            ingredient_id,
            purchase_unit_id,
            quantity_ordered: quantity_ordered.trim(),
            unit_price: unit_price.trim() || '0',
            discount_amount: discount_amount.trim() || '0',
          })),
      }),
    onSuccess: async (result) => {
      setCreateOpen(false)
      resetDraft()
      await queryClient.invalidateQueries({ queryKey: ['purchase-orders'] })
      toast.push('Purchase order dibuat.')
      navigate(`/purchase/${result.data.data.id}`)
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  function resetDraft() {
    setSupplierID('')
    setExpectedDate('')
    setNote('')
    setDiscount('0')
    setLines([
      { key: Date.now(), ingredient_id: '', purchase_unit_id: 0, quantity_ordered: '', unit_price: '', discount_amount: '0' },
    ])
  }

  function patchParams(patch: Record<string, string>) {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(patch)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    if (!('page' in patch)) next.delete('page')
    setParams(next, { replace: true })
  }

  const supplierInfo = (suppliers.data ?? []).find((item) => item.id === supplierID)
  const validLines = lines.filter((line) => line.ingredient_id !== '' && Number(line.quantity_ordered) > 0)
  const canSubmit = supplierID !== '' && validLines.length > 0

  const columns: Column<PurchaseOrder>[] = [
    {
      key: 'code',
      header: 'PO',
      render: (row) => (
        <div className="whitespace-nowrap">
          <p className="num font-medium text-ink">{row.code}</p>
          <p className="text-xs text-muted">{formatDate(row.created_at)}</p>
        </div>
      ),
    },
    {
      key: 'supplier',
      header: 'Supplier',
      render: (row) => (
        <div>
          <p className="text-sm">{row.supplier_name}</p>
          {row.expected_date && (
            <p className="text-xs text-muted">Datang {row.expected_date}</p>
          )}
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={purchaseStatusTone(row.status)}>
          {PO_STATUS_LABELS[row.status] ?? row.status}
        </Badge>
      ),
    },
    {
      key: 'total',
      header: 'Total',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.total_amount)}</span>,
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) => (
        <Link
          to={`/purchase/${row.id}`}
          className="inline-flex min-h-11 items-center rounded-control px-3 text-sm font-medium text-ink hover:bg-raised"
        >
          Detail
        </Link>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        title="Pembelian"
        description="Purchase order dan penerimaan barang. Stok hanya berubah saat penerimaan dicatat."
        action={
          canCreate && (
            <Button
              onClick={() => {
                resetDraft()
                setCreateOpen(true)
              }}
            >
              <Plus className="size-4" strokeWidth={2} />
              PO baru
            </Button>
          )
        }
      />

      <div className="mt-6 flex justify-end">
        <div className="w-full sm:w-60">
          <Select
            ariaLabel="Filter status PO"
            value={status || 'all'}
            onValueChange={(value) => patchParams({ status: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua status' },
              ...PO_STATUSES.map((item) => ({ value: item, label: PO_STATUS_LABELS[item] })),
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={list.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={list.isLoading}
          error={list.isError ? apiMessage(list.error) : undefined}
          onRetry={() => list.refetch()}
          empty="Belum ada purchase order."
        />
        <Pagination
          page={list.data?.meta.page ?? page}
          totalPages={list.data?.meta.total_pages ?? 1}
          total={list.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      <Dialog
        open={createOpen}
        onOpenChange={(next) => !create.isPending && setCreateOpen(next)}
        title="Purchase order baru"
        description="PO dibuat sebagai draft; ajukan lalu setujui sebelum barang bisa diterima."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCreateOpen(false)} disabled={create.isPending}>
              Batal
            </Button>
            <Button loading={create.isPending} disabled={!canSubmit} onClick={() => create.mutate()}>
              Simpan draft
            </Button>
          </>
        }
      >
        <div className="max-h-[70dvh] space-y-4 overflow-y-auto pr-1">
          <Select
            label="Supplier"
            placeholder="Pilih supplier"
            value={supplierID}
            onValueChange={setSupplierID}
            options={(suppliers.data ?? []).map((item) => ({ value: item.id, label: item.name }))}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="Perkiraan datang"
              type="date"
              value={expectedDate}
              onChange={(event) => setExpectedDate(event.target.value)}
            />
            <NumberInput label="Diskon PO" value={discount} onValueChange={setDiscount} />
          </div>
          <Input
            label="Catatan"
            value={note}
            onChange={(event) => setNote(event.target.value)}
          />

          <div>
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium">Item</p>
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  setLines((current) => [
                    ...current,
                    {
                      key: Date.now(),
                      ingredient_id: '',
                      purchase_unit_id: 0,
                      quantity_ordered: '',
                      unit_price: '',
                      discount_amount: '0',
                    },
                  ])
                }
              >
                <Plus className="size-4" strokeWidth={2} />
                Baris
              </Button>
            </div>
            <div className="space-y-3">
              {lines.map((line, index) => {
                const supplierLines = supplierInfo?.ingredients ?? []
                const relation = supplierLines.find(
                  (item) => item.ingredient_id === line.ingredient_id,
                )
                const ingredient = (ingredients.data ?? []).find(
                  (item) => item.id === line.ingredient_id,
                )
                return (
                  <div key={line.key} className="rounded-control border border-line p-3">
                    <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                      <Select
                        ariaLabel={`Bahan baris ${index + 1}`}
                        placeholder={`Pilih bahan baris ${index + 1}`}
                        value={line.ingredient_id}
                        onValueChange={(value) => {
                          const nextRelation = supplierLines.find(
                            (item) => item.ingredient_id === value,
                          )
                          const nextIngredient = (ingredients.data ?? []).find(
                            (item) => item.id === value,
                          )
                          setLines((current) =>
                            current.map((item, i) =>
                              i === index
                                ? {
                                    ...item,
                                    ingredient_id: value,
                                    purchase_unit_id:
                                      nextRelation?.purchase_unit_id ??
                                      nextIngredient?.default_purchase_unit_id ??
                                      nextIngredient?.base_unit_id ??
                                      0,
                                    unit_price: nextRelation?.last_price ?? item.unit_price,
                                  }
                                : item,
                            ),
                          )
                        }}
                        options={(ingredients.data ?? []).map((item) => ({
                          value: item.id,
                          label: `${item.name} · ${item.base_unit_code}`,
                        }))}
                      />
                      <Button
                        variant="ghost"
                        className="text-negative"
                        aria-label={`Hapus baris ${index + 1}`}
                        onClick={() =>
                          setLines((current) => current.filter((_item, i) => i !== index))
                        }
                      >
                        <Trash2 className="size-4" strokeWidth={1.5} />
                      </Button>
                    </div>
                    <div className="mt-2 grid gap-2 sm:grid-cols-[8rem_8rem_minmax(0,1fr)]">
                      <Select
                        ariaLabel={`Satuan beli baris ${index + 1}`}
                        placeholder={`Pilih satuan baris ${index + 1}`}
                        value={line.purchase_unit_id ? String(line.purchase_unit_id) : ''}
                        onValueChange={(value) =>
                          setLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, purchase_unit_id: Number(value) } : item,
                            ),
                          )
                        }
                        options={unitChoices(
                          units.data ?? [],
                          ingredient?.base_unit_id,
                        )}
                      />
                      <Input
                        aria-label={`Jumlah baris ${index + 1}`}
                        inputMode="decimal"
                        placeholder="Jumlah"
                        value={line.quantity_ordered}
                        onChange={(event) =>
                          setLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, quantity_ordered: event.target.value } : item,
                            ),
                          )
                        }
                      />
                      <NumberInput
                        label="Harga satuan"
                        value={line.unit_price}
                        onValueChange={(value) =>
                          setLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, unit_price: value } : item,
                            ),
                          )
                        }
                      />
                    </div>
                    <div className="mt-2 grid gap-2 sm:grid-cols-2">
                      <NumberInput
                        label="Diskon baris"
                        value={line.discount_amount}
                        onValueChange={(value) =>
                          setLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, discount_amount: value } : item,
                            ),
                          )
                        }
                      />
                      {relation && (
                        <p className="self-end text-xs text-muted">
                          Harga terakhir dari supplier{' '}
                          {relation.last_price ? formatMoney(relation.last_price) : 'belum ada'}.
                        </p>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </Dialog>
    </>
  )
}

/**
 * Satuan beli harus berdimensi sama dengan satuan dasar bahan agar konversi ke
 * satuan dasar punya arti (mis. g↔kg, ml↔l, pcs↔lusin).
 */
function unitChoices(units: Unit[], baseUnitID?: number) {
  const base = units.find((unit) => unit.id === baseUnitID)
  return units
    .filter((unit) => !base || unit.dimension === base.dimension)
    .map((unit) => ({
      value: String(unit.id),
      label: unit.is_base ? `${unit.code} (dasar)` : `${unit.code} · ${unit.to_base_factor}`,
    }))
}
