import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { kitchenApi } from '../../api/kitchen'
import type { KitchenTicket, KitchenStatus } from '../../api/kitchen'
import { useSSE } from '../../hooks/useSSE'
import type { SSEConnectionStatus } from '../../hooks/useSSE'

// Payload SSE kitchen.order.created — sesuai backend/internal/sse/events.go.
interface KitchenOrderCreatedPayload {
  order_id: string
  order_no?: string
  queue_no?: number
  service_type: string
  table_name?: string
  round_no: number
  priority: string
  items: {
    name: string
    variant?: string
    modifiers_text?: string
    note?: string
    quantity: number
    station?: string
  }[]
  created_at: string
}

interface ItemStatusChangedPayload {
  order_id: string
  item_id: string
  old_status: string
  new_status: string
  updated_by: string
  timestamp: string
}

interface ItemVoidedPayload {
  order_id: string
  item_id: string
  reason: string
  voided_by: string
  timestamp: string
}

interface OrderStatusChangedPayload {
  order_id: string
  old_status: string
  new_status: string
  timestamp: string
}

/**
 * State KDS: daftar tiket dari REST initial load, lalu update lewat SSE.
 * Event merge ke cache react-query dengan key ['kitchen', status].
 *
 * ponytail: merge optimis per event, bukan full reload; bila event gap
 * (full_reload), panggil invalidate untuk load ulang via REST.
 */
export function useKitchenState(statusFilter: string) {
  const queryClient = useQueryClient()
  const [connStatus, setConnStatus] = useState<SSEConnectionStatus>('disconnected')

  const tickets = useQuery({
    queryKey: ['kitchen', statusFilter],
    queryFn: async () =>
      (await kitchenApi.list(statusFilter ? { status: statusFilter } : {})).data.data,
    refetchOnWindowFocus: false,
  })

  const handleEvent = (event: { type: string; data: unknown }) => {
    switch (event.type) {
      case 'kitchen.order.created': {
        const p = event.data as KitchenOrderCreatedPayload
        // Tambahkan tiket baru ke cache bila status filter cocok atau filter kosong.
        if (!statusFilter || ['NEW', 'COOKING', 'READY'].includes(p.priority === 'RUSH' ? 'NEW' : 'NEW')) {
          const newTicket: KitchenTicket = {
            id: p.order_id,
            order_id: p.order_id,
            order_no: p.order_no,
            queue_no: p.queue_no,
            service_type: p.service_type,
            table_name: p.table_name,
            round_no: p.round_no,
            status: 'NEW',
            priority: p.priority,
            received_at: p.created_at,
            version: 1,
            items: p.items.map((it, i) => ({
              id: `${p.order_id}-${i}`,
              name: it.name,
              variant: it.variant,
              modifiers_text: it.modifiers_text,
              note: it.note,
              quantity: it.quantity,
              station: it.station,
              status: 'NEW',
              version: 1,
            })),
          }
          queryClient.setQueryData(['kitchen', statusFilter], (old: KitchenTicket[] = []) => [
            newTicket,
            ...old,
          ])
        }
        break
      }
      case 'kitchen.item.status_changed': {
        const p = event.data as ItemStatusChangedPayload
        queryClient.setQueryData(['kitchen', statusFilter], (old: KitchenTicket[] = []) =>
          old.map((t) => {
            if (t.order_id !== p.order_id) return t
            const items = t.items.map((it) =>
              it.id === p.item_id || it.order_item_id === p.item_id
                ? { ...it, status: p.new_status as KitchenStatus, version: it.version + 1 }
                : it,
            )
            // Re-compute ticket status sederhana.
            const ticketStatus = computeTicketStatus(items.map((i) => i.status))
            return { ...t, items, status: ticketStatus, version: t.version + 1 }
          }),
        )
        break
      }
      case 'kitchen.item.voided': {
        const p = event.data as ItemVoidedPayload
        queryClient.setQueryData(['kitchen', statusFilter], (old: KitchenTicket[] = []) =>
          old
            .map((t) => {
              if (t.order_id !== p.order_id) return t
              const items = t.items.map((it) =>
                it.id === p.item_id || it.order_item_id === p.item_id
                  ? { ...it, status: 'VOID' as KitchenStatus }
                  : it,
              )
              const ticketStatus = computeTicketStatus(items.map((i) => i.status))
              return { ...t, items, status: ticketStatus }
            })
            .filter((t) => t.status !== 'VOID' && t.status !== 'SERVED'),
        )
        break
      }
      case 'order.status_changed': {
        const p = event.data as OrderStatusChangedPayload
        if (p.new_status === 'COMPLETED' || p.new_status === 'CANCELLED') {
          queryClient.setQueryData(['kitchen', statusFilter], (old: KitchenTicket[] = []) =>
            old.filter((t) => t.order_id !== p.order_id),
          )
        }
        break
      }
      case 'full_reload':
        void queryClient.invalidateQueries({ queryKey: ['kitchen'] })
        break
    }
  }

  useSSE({
    url: '/api/v1/kitchen/stream',
    onEvent: (e) => handleEvent(e as { type: string; data: unknown }),
    onFullReload: () => void queryClient.invalidateQueries({ queryKey: ['kitchen'] }),
    onConnectionChange: setConnStatus,
  })

  return { tickets, connStatus, refetch: () => void tickets.refetch() }
}

function computeTicketStatus(itemStatuses: string[]): KitchenStatus {
  const active = itemStatuses.filter((s) => s !== 'VOID')
  if (active.length === 0) return 'VOID'
  if (active.every((s) => s === 'SERVED')) return 'SERVED'
  if (active.every((s) => s === 'READY' || s === 'SERVED')) return 'READY'
  if (active.some((s) => s === 'COOKING' || s === 'READY' || s === 'SERVED')) return 'COOKING'
  return 'NEW'
}
