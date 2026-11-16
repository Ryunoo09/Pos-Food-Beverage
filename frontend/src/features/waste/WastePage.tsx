import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { inventoryApi } from '../../api/inventory'
import { menuApi } from '../../api/menu'
import {
  WASTE_REASONS,
  WASTE_TYPES,
  WASTE_TYPE_LABELS,
  wasteApi,
  wasteReasonLabel,
} from '../../api/waste'
import type { WasteRecord, WasteType } from '../../api/waste'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { wasteStatusTone } from '../../utils/status'

const perPage = 20

export function WastePage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('waste.create')
  const canReverse = permissions.includes('waste.reverse')

  const [params, setParams] = useSearchParams()
  const type = params.get('waste_type') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [createOpen, setCreateOpen] = useState(false)
  const [reversing, setReversing] = useState<WasteRecord | null>(null)

  const list = useQuery({
    queryKey: ['waste-records', { type, page }],
    queryFn: async () =>
      (
        await wasteApi.list({
          waste_type: type || undefined,
          page,
          per_page: perPage,
        })
      ).data,
  })

  const create = useMutation({
    mutationFn: (input: {
      waste_type: WasteType
      ingredient_id: string | null
      variant_id: string | null
      quantity: string
      reason_code: string | null
      note: string | null
    }) =>
      wasteApi.create({
        waste_type: input.waste_type,
        ingredient_id: input.ingredient_id,
        variant_id: input.variant_id,
        quantity: input.quantity,
        unit_id: null,
        reason_code: input.reason_code,
        note: input.note,
        order_id: null,
      }),
    onSuccess: async () => {
      setCreateOpen(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['waste-records'] }),
        queryClient.invalidateQueries({ queryKey: ['stock'] }),
      ])
      toast.push('Waste dicatat.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const reverse = useMutation({
    mutationFn: (id: string) => wasteApi.reverse(id),
    onSuccess: async () => {
      setReversing(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['waste-records'] }),
        queryClient.invalidateQueries({ queryKey: ['stock'] }),
      ])
      toast.push('Waste dibatalkan dan stok dikembalikan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
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

  const columns: Column<WasteRecord>[] = [
    {
      key: 'item',
      header: 'Item',
      render: (row) => (
        <div className="whitespace-nowrap">
          <p className="font-medium text-ink">
            {row.ingredient_name ?? row.variant_name ?? '-'}
          </p>
          <p className="text-xs text-muted">
            {WASTE_TYPE_LABELS[row.waste_type] ?? row.waste_type} · {formatDate(row.recorded_at)}
          </p>
        </div>
      ),
    },
    {
      key: 'quantity',
      header: 'Jumlah',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num">
          {row.quantity} {row.unit_code ?? ''}
        </span>
      ),
    },
    {
      key: 'cost',
      header: 'Nilai',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.cost_value)}</span>,
    },
    {
      key: 'reason',
      header: 'Alasan',
      render: (row) => (
        <div>
          <p className="text-sm">{wasteReasonLabel(row.reason_code)}</p>
          {row.note && <p className="text-xs text-muted">{row.note}</p>}
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={wasteStatusTone(row.status)}>
          {row.status === 'REVERSED' ? 'Dibatalkan' : 'Tercatat'}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) =>
        canReverse && row.status !== 'REVERSED' ? (
          <Button
            variant="ghost"
            size="sm"
            className="text-negative"
            onClick={() => setReversing(row)}
          >
            <RotateCcw className="size-4" strokeWidth={1.5} />
            Batalkan
          </Button>
        ) : null,
    },
  ]

  return (
    <>
      <PageHeader
        title="Waste"
        description="Bahan atau produk terbuang. Pencatatan langsung mengurangi stok dan mencatat nilainya."
        action={
          canCreate && (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" strokeWidth={2} />
              Catat waste
            </Button>
          )
        }
      />

      <div className="mt-6 flex justify-end">
        <div className="w-full sm:w-52">
          <Select
            ariaLabel="Filter jenis waste"
            value={type || 'all'}
            onValueChange={(value) => patchParams({ waste_type: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua jenis' },
              ...WASTE_TYPES.map((item) => ({ value: item, label: WASTE_TYPE_LABELS[item] })),
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
          empty="Belum ada pencatatan waste."
        />
        <Pagination
          page={list.data?.meta.page ?? page}
          totalPages={list.data?.meta.total_pages ?? 1}
          total={list.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      {createOpen && (
        <CreateWasteDialog
          open={createOpen}
          loading={create.isPending}
          onOpenChange={(open) => !create.isPending && setCreateOpen(open)}
          onSubmit={(input) => create.mutate(input)}
        />
      )}

      <ConfirmDialog
        open={reversing !== null}
        title="Batalkan pencatatan waste?"
        description="Movement pembalik akan dibuat dan stok kembali seperti sebelum pencatatan. Catatan asli tetap tersimpan sebagai riwayat."
        confirmLabel="Batalkan waste"
        loading={reverse.isPending}
        onCancel={() => setReversing(null)}
        onConfirm={() => reversing && reverse.mutate(reversing.id)}
      />
    </>
  )
}

function CreateWasteDialog({
  open,
  loading,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  loading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: {
    waste_type: WasteType
    ingredient_id: string | null
    variant_id: string | null
    quantity: string
    reason_code: string | null
    note: string | null
  }) => void
}) {
  const [wasteType, setWasteType] = useState<WasteType>('INGREDIENT')
  const [ingredientID, setIngredientID] = useState('')
  const [variantID, setVariantID] = useState('')
  const [quantity, setQuantity] = useState('')
  const [reason, setReason] = useState<string>('')
  const [note, setNote] = useState('')

  const ingredients = useQuery({
    queryKey: ['ingredients', { active: true, q: '' }],
    queryFn: async () =>
      (await inventoryApi.ingredients.list({ is_active: true, per_page: 100 })).data.data,
  })
  const products = useQuery({
    queryKey: ['pos-products'],
    queryFn: async () => (await menuApi.products.list({ is_active: true, per_page: 100 })).data.data,
  })

  const variantChoices = (products.data ?? []).flatMap((product) =>
    (product.variants ?? [])
      .filter((variant) => variant.is_active)
      .map((variant) => ({
        value: variant.id,
        label: `${product.name} · ${variant.name}`,
      })),
  )

  const targetChosen = wasteType === 'INGREDIENT' ? ingredientID !== '' : variantID !== ''
  const quantityValid = /^\d+(\.\d{1,3})?$/.test(quantity.trim()) && Number(quantity) > 0
  const canSubmit = targetChosen && quantityValid

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Catat waste"
      description="Pengurangan stok dihitung server dari resep atau bahan stok produk."
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button
            loading={loading}
            disabled={!canSubmit}
            onClick={() =>
              onSubmit({
                waste_type: wasteType,
                ingredient_id: wasteType === 'INGREDIENT' ? ingredientID : null,
                variant_id: wasteType === 'PRODUCT' ? variantID : null,
                quantity: quantity.trim(),
                reason_code: reason || null,
                note: note.trim() || null,
              })
            }
          >
            Simpan
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Select
          label="Jenis"
          value={wasteType}
          onValueChange={(value) => {
            setWasteType(value as WasteType)
            setIngredientID('')
            setVariantID('')
          }}
          options={[
            { value: 'INGREDIENT', label: 'Bahan' },
            { value: 'PRODUCT', label: 'Produk jadi' },
          ]}
        />

        {wasteType === 'INGREDIENT' ? (
          <Select
            label="Bahan"
            placeholder="Pilih bahan"
            value={ingredientID}
            onValueChange={setIngredientID}
            options={(ingredients.data ?? []).map((item) => ({
              value: item.id,
              label: `${item.name} · ${item.base_unit_code}`,
            }))}
          />
        ) : (
          <Select
            label="Produk"
            placeholder="Pilih varian produk"
            value={variantID}
            onValueChange={setVariantID}
            options={variantChoices}
          />
        )}

        <Input
          label="Jumlah"
          inputMode="decimal"
          value={quantity}
          onChange={(event) => setQuantity(event.target.value)}
          helper={
            wasteType === 'INGREDIENT'
              ? 'Dalam satuan dasar bahan.'
              : 'Dalam porsi produk.'
          }
        />
        <Select
          label="Alasan"
          placeholder="Pilih alasan"
          value={reason}
          onValueChange={setReason}
          options={WASTE_REASONS.map((item) => ({ value: item.value, label: item.label }))}
        />
        <Input
          label="Catatan"
          value={note}
          onChange={(event) => setNote(event.target.value)}
        />
      </div>
    </Dialog>
  )
}
