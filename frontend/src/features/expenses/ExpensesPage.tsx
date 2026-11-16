import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Ban, Plus, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { PAYMENT_METHOD_LABELS, PAYMENT_METHOD_OPTIONS } from '../../api/dashboard'
import { expenseApi } from '../../api/expenses'
import type { Expense, ExpenseInput } from '../../api/expenses'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { DateRangePicker } from '../../components/ui/DateRangePicker'
import type { DateRange } from '../../components/ui/DateRangePicker'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'
import { ExpenseCategoriesDialog } from './ExpenseCategoriesDialog'

const perPage = 20

function todayISO(): string {
  const now = new Date()
  return new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
}

function monthStartISO(): string {
  const now = new Date()
  return new Date(now.getFullYear(), now.getMonth(), 1)
    .toLocaleDateString('sv-SE')
    .slice(0, 10)
}

export function ExpensesPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('expenses.create')
  const canVoid = permissions.includes('expenses.void')
  const canManageCategories = permissions.includes('expense_categories.manage')

  const [params, setParams] = useSearchParams()
  const from = params.get('from') || monthStartISO()
  const to = params.get('to') || todayISO()
  const categoryID = params.get('category_id') ?? ''
  const includeVoided = params.get('include_voided') === 'true'
  const page = Math.max(1, Number(params.get('page')) || 1)

  const [createOpen, setCreateOpen] = useState(false)
  const [categoriesOpen, setCategoriesOpen] = useState(false)
  const [voiding, setVoiding] = useState<Expense | null>(null)

  const categories = useQuery({
    queryKey: ['expense-categories'],
    queryFn: async () => (await expenseApi.categories.list()).data.data,
  })

  const list = useQuery({
    queryKey: ['expenses', { from, to, categoryID, includeVoided, page }],
    queryFn: async () =>
      (
        await expenseApi.list({
          from,
          to,
          category_id: categoryID || undefined,
          include_voided: includeVoided || undefined,
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

  const create = useMutation({
    mutationFn: (input: ExpenseInput) => expenseApi.create(input),
    onSuccess: async () => {
      setCreateOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['expenses'] })
      toast.push('Pengeluaran dicatat.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const voidExpense = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => expenseApi.void(id, reason),
    onSuccess: async () => {
      setVoiding(null)
      await queryClient.invalidateQueries({ queryKey: ['expenses'] })
      toast.push('Pengeluaran dibatalkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const rangeValue: DateRange = { from, to }

  const columns: Column<Expense>[] = [
    {
      key: 'name',
      header: 'Pengeluaran',
      render: (row) => (
        <div className="min-w-0">
          <p className={row.is_voided ? 'font-medium text-muted line-through' : 'font-medium text-ink'}>
            {row.name}
          </p>
          <p className="text-xs text-muted">
            {row.category_name} · {row.expense_date}
          </p>
        </div>
      ),
    },
    {
      key: 'method',
      header: 'Pembayaran',
      render: (row) => {
        const method = row.payment_method_code
          ? (PAYMENT_METHOD_LABELS[row.payment_method_code] ?? row.payment_method_code)
          : null
        // Pengeluaran dari laci sering tidak punya kode metode: sumber dananya
        // adalah kas shift, bukan metode bayar.
        if (row.paid_from_drawer) {
          return (
            <div className="whitespace-nowrap text-sm">
              <p>Kas laci</p>
              {method && <p className="text-xs text-muted">{method}</p>}
            </div>
          )
        }
        return <p className="whitespace-nowrap text-sm">{method ?? '—'}</p>
      },
    },
    {
      key: 'amount',
      header: 'Nominal',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num tabular-nums">{formatMoney(row.amount)}</span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) =>
        row.is_voided ? (
          <div>
            <Badge tone="gray">Dibatalkan</Badge>
            {row.void_reason && <p className="mt-1 text-xs text-muted">{row.void_reason}</p>}
          </div>
        ) : (
          <Badge tone="green">Tercatat</Badge>
        ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) =>
        canVoid && !row.is_voided ? (
          <Button
            variant="ghost"
            size="sm"
            className="text-negative"
            onClick={() => setVoiding(row)}
          >
            <Ban className="size-4" strokeWidth={1.5} />
            Batalkan
          </Button>
        ) : null,
    },
  ]

  return (
    <>
      <PageHeader
        title="Pengeluaran"
        description="Biaya operasional di luar bahan baku: listrik, sewa, gaji, dan sejenisnya."
        action={
          <div className="flex items-center gap-2">
            {canManageCategories && (
              <Button variant="secondary" onClick={() => setCategoriesOpen(true)}>
                <Settings2 className="size-4" strokeWidth={1.5} />
                Kategori
              </Button>
            )}
            {canCreate && (
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" strokeWidth={2} />
                Catat pengeluaran
              </Button>
            )}
          </div>
        }
      />

      <div className="mt-6 flex flex-wrap items-end gap-3">
        <div className="w-full sm:w-auto">
          <DateRangePicker
            value={rangeValue}
            onChange={(next) => patchParams({ from: next.from, to: next.to })}
          />
        </div>
        <div className="w-full sm:w-52">
          <Select
            ariaLabel="Filter kategori"
            value={categoryID || 'all'}
            onValueChange={(value) => patchParams({ category_id: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua kategori' },
              ...(categories.data ?? []).map((item) => ({ value: item.id, label: item.name })),
            ]}
          />
        </div>
        <label className="flex min-h-11 items-center gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="size-4 accent-brand-600"
            checked={includeVoided}
            onChange={(event) =>
              patchParams({ include_voided: event.target.checked ? 'true' : '' })
            }
          />
          Tampilkan yang dibatalkan
        </label>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={list.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={list.isLoading}
          error={list.isError ? apiMessage(list.error) : undefined}
          onRetry={() => list.refetch()}
          empty="Belum ada pengeluaran pada periode ini."
        />
        <Pagination
          page={list.data?.meta.page ?? page}
          totalPages={list.data?.meta.total_pages ?? 1}
          total={list.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      {createOpen && (
        <CreateExpenseDialog
          open={createOpen}
          loading={create.isPending}
          categories={categories.data ?? []}
          onOpenChange={(open) => !create.isPending && setCreateOpen(open)}
          onSubmit={(input) => create.mutate(input)}
        />
      )}

      <ExpenseCategoriesDialog
        open={categoriesOpen}
        onOpenChange={setCategoriesOpen}
        categories={categories.data ?? []}
      />

      {voiding && (
        <VoidExpenseDialog
          expense={voiding}
          loading={voidExpense.isPending}
          onCancel={() => setVoiding(null)}
          onConfirm={(reason) => voidExpense.mutate({ id: voiding.id, reason })}
        />
      )}
    </>
  )
}

function CreateExpenseDialog({
  open,
  loading,
  categories,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  loading: boolean
  categories: { id: string; name: string; is_active: boolean }[]
  onOpenChange: (open: boolean) => void
  onSubmit: (input: ExpenseInput) => void
}) {
  const [categoryID, setCategoryID] = useState('')
  const [name, setName] = useState('')
  const [amount, setAmount] = useState('')
  const [date, setDate] = useState(todayISO())
  const [method, setMethod] = useState('CASH')
  const [paidFromDrawer, setPaidFromDrawer] = useState(false)
  const [note, setNote] = useState('')

  const activeCategories = categories.filter((item) => item.is_active)
  const amountValid = /^\d+$/.test(amount) && Number(amount) > 0
  const canSubmit = categoryID !== '' && name.trim() !== '' && amountValid && date !== ''

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Catat pengeluaran"
      description="Nominal disimpan sebagai angka bulat rupiah."
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
                category_id: categoryID,
                name: name.trim(),
                amount,
                expense_date: date,
                payment_method_code: method || null,
                paid_from_drawer: paidFromDrawer,
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
        {activeCategories.length === 0 ? (
          <p className="rounded-control bg-raised px-3 py-2 text-sm text-muted">
            Belum ada kategori aktif. Tambahkan kategori lebih dulu lewat tombol Kategori.
          </p>
        ) : (
          <Select
            label="Kategori"
            placeholder="Pilih kategori"
            value={categoryID}
            onValueChange={setCategoryID}
            options={activeCategories.map((item) => ({ value: item.id, label: item.name }))}
          />
        )}
        <Input
          label="Nama pengeluaran"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Token listrik"
        />
        <NumberInput label="Nominal" value={amount} onValueChange={setAmount} />
        <Input
          label="Tanggal"
          type="date"
          value={date}
          max={todayISO()}
          onChange={(event) => setDate(event.target.value)}
          helper="Maksimal 1 tahun ke belakang."
        />
        <Select
          label="Metode bayar"
          value={method}
          onValueChange={setMethod}
          options={PAYMENT_METHOD_OPTIONS.map((item) => ({ value: item.value, label: item.label }))}
        />
        <label className="flex items-start gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="mt-0.5 size-4 accent-brand-600"
            checked={paidFromDrawer}
            onChange={(event) => setPaidFromDrawer(event.target.checked)}
          />
          <span>
            Dibayar dari laci kasir
            <span className="block text-xs text-muted">
              Memerlukan shift terbuka; kas keluar akan tercatat di shift.
            </span>
          </span>
        </label>
        <Input
          label="Catatan"
          value={note}
          onChange={(event) => setNote(event.target.value)}
          placeholder="Opsional"
        />
      </div>
    </Dialog>
  )
}

function VoidExpenseDialog({
  expense,
  loading,
  onCancel,
  onConfirm,
}: {
  expense: Expense
  loading: boolean
  onCancel: () => void
  onConfirm: (reason: string) => void
}) {
  const [reason, setReason] = useState('')

  return (
    <Dialog
      open
      onOpenChange={(next) => !next && !loading && onCancel()}
      title="Batalkan pengeluaran?"
      description={`${expense.name} · ${formatMoney(expense.amount)}. Catatan tetap tersimpan sebagai riwayat.`}
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={loading}>
            Batal
          </Button>
          <Button
            variant="danger"
            loading={loading}
            disabled={reason.trim() === ''}
            onClick={() => onConfirm(reason.trim())}
          >
            Batalkan pengeluaran
          </Button>
        </>
      }
    >
      <Input
        label="Alasan pembatalan"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        helper="Wajib diisi untuk jejak audit."
      />
    </Dialog>
  )
}
