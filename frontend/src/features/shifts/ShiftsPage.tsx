import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { shiftApi } from '../../api/shifts'
import type { Shift } from '../../api/shifts'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { DataTable } from '../../components/ui/DataTable'
import type { Column } from '../../components/ui/DataTable'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Pagination } from '../../components/ui/Pagination'
import { Select } from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'

const perPage = 20

export function ShiftsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const status = params.get('status') ?? ''
  const page = Math.max(1, Number(params.get('page')) || 1)
  const [openDialog, setOpenDialog] = useState(false)
  const [openingCash, setOpeningCash] = useState('0')

  const current = useQuery({
    queryKey: ['shift-current'],
    queryFn: async () => (await shiftApi.current()).data.data,
    retry: false,
  })

  const shifts = useQuery({
    queryKey: ['shifts', { status, page }],
    queryFn: async () =>
      (await shiftApi.list({ status: status || undefined, page, per_page: perPage })).data,
  })

  const open = useMutation({
    mutationFn: (amount: string) => shiftApi.open(amount),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['shift-current'] }),
        queryClient.invalidateQueries({ queryKey: ['shifts'] }),
      ])
      setOpenDialog(false)
      setOpeningCash('0')
      toast.push('Shift dibuka.')
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

  const columns: Column<Shift>[] = [
    {
      key: 'opened',
      header: 'Dibuka',
      render: (row) => (
        <div>
          <p className="font-medium">{formatDate(row.opened_at)}</p>
          <p className="text-xs text-muted">
            {row.closed_at ? `Ditutup ${formatDate(row.closed_at)}` : 'Masih berjalan'}
          </p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (
        <Badge tone={row.status === 'OPEN' ? 'green' : 'gray'}>
          {row.status === 'OPEN' ? 'Terbuka' : 'Ditutup'}
        </Badge>
      ),
    },
    {
      key: 'opening',
      header: 'Modal awal',
      className: 'text-right whitespace-nowrap',
      render: (row) => <span className="num">{formatMoney(row.opening_cash)}</span>,
    },
    {
      key: 'expected',
      header: 'Kas seharusnya',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num">{row.expected_cash ? formatMoney(row.expected_cash) : '-'}</span>
      ),
    },
    {
      key: 'actual',
      header: 'Kas aktual',
      className: 'text-right whitespace-nowrap',
      render: (row) => (
        <span className="num">{row.actual_cash ? formatMoney(row.actual_cash) : '-'}</span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'text-right',
      render: (row) => (
        <Link
          to={`/shifts/${row.id}`}
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
        title="Shift kasir"
        description="Rekonsiliasi kas per shift: modal awal, penjualan tunai, dan selisih saat tutup."
        action={
          !current.data && (
            <Button onClick={() => setOpenDialog(true)}>
              <Plus className="size-4" strokeWidth={2} />
              Buka shift
            </Button>
          )
        }
      />

      {current.data && (
        <Card className="mt-6">
          <CardHeader
            title="Shift aktif"
            action={
              <Link to={`/shifts/${current.data.id}`}>
                <Button variant="secondary" size="sm">
                  Buka detail
                </Button>
              </Link>
            }
          />
          <CardBody className="grid gap-4 sm:grid-cols-3">
            <div>
              <p className="text-xs text-muted">Dibuka</p>
              <p className="text-sm font-medium">{formatDate(current.data.opened_at)}</p>
            </div>
            <div>
              <p className="text-xs text-muted">Modal awal</p>
              <p className="num text-sm font-medium">{formatMoney(current.data.opening_cash)}</p>
            </div>
            <div>
              <p className="text-xs text-muted">Status</p>
              <Badge tone="green">Terbuka</Badge>
            </div>
          </CardBody>
        </Card>
      )}

      <div className="mt-6 flex justify-end">
        <div className="w-full sm:w-52">
          <Select
            aria-label="Filter status shift"
            value={status || 'all'}
            onValueChange={(value) => patchParams({ status: value === 'all' ? '' : value })}
            options={[
              { value: 'all', label: 'Semua status' },
              { value: 'OPEN', label: 'Terbuka' },
              { value: 'CLOSED', label: 'Ditutup' },
            ]}
          />
        </div>
      </div>

      <Card className="mt-4">
        <DataTable
          columns={columns}
          rows={shifts.data?.data ?? []}
          rowKey={(row) => row.id}
          loading={shifts.isLoading}
          error={shifts.isError ? apiMessage(shifts.error) : undefined}
          onRetry={() => shifts.refetch()}
          empty="Belum ada shift."
        />
        <Pagination
          page={shifts.data?.meta.page ?? page}
          totalPages={shifts.data?.meta.total_pages ?? 1}
          total={shifts.data?.meta.total ?? 0}
          onPageChange={(next) => patchParams({ page: String(next) })}
        />
      </Card>

      <Dialog
        open={openDialog}
        onOpenChange={(next) => !open.isPending && setOpenDialog(next)}
        title="Buka shift"
        description="Masukkan modal kas awal. Satu kasir hanya boleh punya satu shift terbuka."
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpenDialog(false)} disabled={open.isPending}>
              Batal
            </Button>
            <Button
              loading={open.isPending}
              onClick={() => open.mutate(openingCash.trim() || '0')}
              disabled={!/^\d+(\.\d+)?$/.test(openingCash.trim())}
            >
              Buka shift
            </Button>
          </>
        }
      >
        <Input
          label="Modal awal"
          inputMode="decimal"
          value={openingCash}
          onChange={(event) => setOpeningCash(event.target.value)}
          helper="Boleh 0 bila kas awal kosong."
        />
      </Dialog>
    </>
  )
}
