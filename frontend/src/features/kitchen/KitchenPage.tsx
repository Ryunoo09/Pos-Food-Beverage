import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Flame, RefreshCw, Wifi, WifiOff } from 'lucide-react'
import { useState } from 'react'
import {
  KITCHEN_STATUS_LABELS,
  kitchenApi,
} from '../../api/kitchen'
import type { KitchenItem, KitchenStatus, KitchenTicket } from '../../api/kitchen'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { useKitchenState } from './useKitchenState'

const FILTERS = [
  { value: '', label: 'Semua aktif' },
  { value: 'NEW', label: 'Baru' },
  { value: 'COOKING', label: 'Dimasak' },
  { value: 'READY', label: 'Siap' },
] as const

const NEXT_STATUS: Partial<Record<KitchenStatus, KitchenStatus>> = {
  NEW: 'COOKING',
  COOKING: 'READY',
  READY: 'SERVED',
}

export function KitchenPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const { tickets, connStatus, refetch } = useKitchenState(status)

  const update = useMutation({
    mutationFn: ({ item, next }: { item: KitchenItem; next: KitchenStatus }) =>
      kitchenApi.updateItemStatus(item.id, next, item.version),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['kitchen'] })
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const priority = useMutation({
    mutationFn: ({ ticket, next }: { ticket: KitchenTicket; next: 'NORMAL' | 'RUSH' }) =>
      kitchenApi.setPriority(ticket.order_id, next),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['kitchen'] })
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const rows = [...(tickets.data ?? [])].sort((a, b) => {
    if (a.priority !== b.priority) return a.priority === 'RUSH' ? -1 : 1
    return new Date(a.received_at).getTime() - new Date(b.received_at).getTime()
  })

  async function setAllReady(ticket: KitchenTicket) {
    const pending = ticket.items.filter((item) => item.status === 'NEW' || item.status === 'COOKING')
    if (pending.length === 0) return
    setBusy(ticket.id)
    try {
      for (const item of pending) {
        await kitchenApi.updateItemStatus(item.id, 'READY', item.version)
      }
      await queryClient.invalidateQueries({ queryKey: ['kitchen'] })
      toast.push(`Tiket ${ticket.order_no ?? ''} siap disajikan.`)
    } catch (error) {
      toast.push(apiMessage(error), 'error')
      await queryClient.invalidateQueries({ queryKey: ['kitchen'] })
    } finally {
      setBusy(null)
    }
  }

  const connLabel =
    connStatus === 'connected' ? 'Realtime tersambung' :
    connStatus === 'reconnecting' ? 'Menghubungkan ulang…' :
    'Realtime terputus'

  return (
    <div className="flex min-h-[calc(100dvh-8rem)] flex-col">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((filter) => (
            <button
              key={filter.value}
              type="button"
              onClick={() => setStatus(filter.value)}
              className={cn(
                'min-h-11 rounded-control border px-4 text-sm font-medium',
                status === filter.value
                  ? 'border-ink bg-ink text-surface'
                  : 'border-control bg-surface text-ink hover:bg-raised',
              )}
            >
              {filter.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-3 text-sm text-muted">
          {tickets.isFetching && <RefreshCw className="size-4 animate-spin" strokeWidth={1.5} />}
          <span className={cn(
            'inline-flex items-center gap-1.5',
            connStatus === 'connected' ? 'text-positive' : connStatus === 'reconnecting' ? 'text-warning' : 'text-negative',
          )}>
            {connStatus === 'connected'
              ? <Wifi className="size-4" strokeWidth={1.5} />
              : <WifiOff className="size-4" strokeWidth={1.5} />}
            {connLabel}
          </span>
          <Button variant="secondary" size="sm" onClick={refetch}>
            Muat ulang
          </Button>
        </div>
      </div>

      {tickets.isLoading ? (
        <div className="grid flex-1 place-items-center">
          <Spinner label="Memuat tiket dapur" />
        </div>
      ) : tickets.isError ? (
        <p className="py-10 text-sm text-negative">{apiMessage(tickets.error)}</p>
      ) : rows.length === 0 ? (
        <p className="py-16 text-center text-base text-muted">Tidak ada tiket aktif.</p>
      ) : (
        <div className="mt-4 grid flex-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {rows.map((ticket) => {
            const age = ageMinutes(ticket.received_at)
            const tone = age < 5 ? 'border-positive' : age <= 10 ? 'border-warning' : 'border-negative'
            return (
              <article
                key={ticket.id}
                className={cn('flex flex-col rounded-card border-t-4 border border-line bg-surface', tone)}
              >
                <header className="flex items-start justify-between gap-2 border-b border-rule px-4 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-base font-semibold">
                      {ticket.order_no ?? ticket.order_id.slice(0, 8)}
                    </p>
                    <p className="text-sm text-muted">
                      {ticket.table_name ? `Meja ${ticket.table_name}` : ticket.service_type}
                      {ticket.queue_no ? ` · #${ticket.queue_no}` : ''}
                      {ticket.round_no > 1 ? ` · Ronde ${ticket.round_no}` : ''}
                    </p>
                  </div>
                  <div className="text-right">
                    <span
                      className={cn(
                        'num text-lg font-semibold',
                        age < 5 ? 'text-positive' : age <= 10 ? 'text-warning' : 'text-negative',
                      )}
                    >
                      {ageLabel(age)}
                    </span>
                    <p className="text-xs text-muted uppercase">{ticket.status}</p>
                  </div>
                </header>

                <ul className="flex-1 divide-y divide-rule">
                  {ticket.items.map((item) => (
                    <li key={item.id} className="px-4 py-3">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="text-base leading-snug font-medium">
                            <span className="num mr-1">{item.quantity}×</span>
                            {item.name}
                          </p>
                          {item.variant && <p className="text-sm text-muted">{item.variant}</p>}
                          {item.modifiers_text && (
                            <p className="text-sm text-muted">{item.modifiers_text}</p>
                          )}
                          {item.note && (
                            <p className="text-sm font-medium text-warning">Catatan: {item.note}</p>
                          )}
                          <Badge tone={item.status === 'READY' ? 'green' : item.status === 'NEW' ? 'gray' : item.status === 'VOID' ? 'red' : 'yellow'}>
                            {KITCHEN_STATUS_LABELS[item.status]}
                          </Badge>
                        </div>
                        {item.status !== 'VOID' && NEXT_STATUS[item.status] && (
                          <Button
                            size="sm"
                            variant={item.status === 'COOKING' ? 'primary' : 'secondary'}
                            onClick={() =>
                              update.mutate({ item, next: NEXT_STATUS[item.status] as KitchenStatus })
                            }
                            disabled={update.isPending || busy === ticket.id}
                          >
                            {item.status === 'NEW' ? 'Mulai' : item.status === 'COOKING' ? 'Siap' : 'Sajikan'}
                          </Button>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>

                <footer className="flex flex-wrap items-center gap-2 border-t border-rule px-4 py-3">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() =>
                      priority.mutate({
                        ticket,
                        next: ticket.priority === 'RUSH' ? 'NORMAL' : 'RUSH',
                      })
                    }
                    disabled={priority.isPending}
                  >
                    <Flame className="size-4" strokeWidth={1.5} />
                    {ticket.priority === 'RUSH' ? 'Batalkan RUSH' : 'Tandai RUSH'}
                  </Button>
                  <Button
                    className="ml-auto"
                    size="sm"
                    loading={busy === ticket.id}
                    onClick={() => setAllReady(ticket)}
                    disabled={ticket.items.every((item) => item.status === 'READY' || item.status === 'SERVED' || item.status === 'VOID')}
                  >
                    Semua siap
                  </Button>
                </footer>
              </article>
            )
          })}
        </div>
      )}
    </div>
  )
}

function ageMinutes(iso: string): number {
  const elapsed = Date.now() - new Date(iso).getTime()
  return Math.max(0, Math.floor(elapsed / 60_000))
}

function ageLabel(minutes: number): string {
  return `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`
}
