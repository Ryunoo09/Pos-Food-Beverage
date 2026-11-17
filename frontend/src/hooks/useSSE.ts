import { useEffect, useRef } from 'react'
import { useAuthStore } from '../store/auth'

export interface SSEEvent {
  id: number
  type: string
  data: unknown
}

export type SSEConnectionStatus = 'connected' | 'disconnected' | 'reconnecting'

interface SSEOptions {
  /** URL stream relatif (mis. `/api/v1/kitchen/stream`). Token diambil dari auth store. */
  url: string
  onEvent: (event: SSEEvent) => void
  onFullReload?: () => void
  onConnectionChange?: (status: SSEConnectionStatus) => void
  /** Last event id untuk replay; 0 = baru. */
  lastEventId?: number
}

const MAX_DELAY = 15_000

/**
 * Hook SSE dengan EventSource, backoff reconnect 1/2/4/8/15 detik,
 * dan Last-Event-ID via query param `last_event_id` (EventSource tidak bisa
 * set header custom, jadi token dikirim sebagai query param `token`).
 *
 * Token akses diambil dari auth store. Bila token hilang, koneksi ditutup.
 */
export function useSSE({
  url,
  onEvent,
  onFullReload,
  onConnectionChange,
  lastEventId,
}: SSEOptions) {
  const token = useAuthStore((s) => s.accessToken)
  const handlerRef = useRef(onEvent)
  const fullReloadRef = useRef(onFullReload)
  const connChangeRef = useRef(onConnectionChange)

  useEffect(() => {
    handlerRef.current = onEvent
  })
  useEffect(() => {
    fullReloadRef.current = onFullReload
  })
  useEffect(() => {
    connChangeRef.current = onConnectionChange
  })

  // Koneksi dibuat sekali per (url, token, lastEventId); callback lewat ref
  // agar perubahan callback di parent tidak trigger reconnect.
  useEffect(() => {
    let source: EventSource | null = null
    let attempt = 0
    let timer: number | undefined
    let closed = false
    let currentLastId = lastEventId ?? 0

    function open() {
      if (closed) return
      const target = new URL(url, window.location.origin)
      if (token) target.searchParams.set('token', token)
      if (currentLastId > 0) target.searchParams.set('last_event_id', String(currentLastId))

      source = new EventSource(target.toString())

      source.onopen = () => {
        attempt = 0
        connChangeRef.current?.('connected')
      }

      const eventNames = [
        'kitchen.order.created',
        'kitchen.item.status_changed',
        'kitchen.item.voided',
        'order.status_changed',
        'order.payment_status_changed',
        'table.status_changed',
        'ping',
        'full_reload',
      ]
      for (const name of eventNames) {
        source.addEventListener(name, (e: MessageEvent) => {
          if (name === 'full_reload') {
            fullReloadRef.current?.()
            return
          }
          if (name === 'ping') return
          if (e.lastEventId) currentLastId = parseInt(e.lastEventId, 10)
          let data: unknown = null
          if (e.data) {
            try {
              data = JSON.parse(e.data)
            } catch {
              data = e.data
            }
          }
          handlerRef.current({
            id: currentLastId,
            type: name,
            data,
          })
        })
      }

      source.onmessage = (e: MessageEvent) => {
        if (e.lastEventId) currentLastId = parseInt(e.lastEventId, 10)
        let data: unknown = null
        if (e.data) {
          try {
            data = JSON.parse(e.data)
          } catch {
            data = e.data
          }
        }
        handlerRef.current({ id: currentLastId, type: 'message', data })
      }

      source.onerror = () => {
        source?.close()
        if (closed) return
        connChangeRef.current?.('reconnecting')
        const delay = Math.min(MAX_DELAY, 1000 * 2 ** attempt)
        attempt += 1
        timer = window.setTimeout(open, delay)
      }
    }

    open()

    return () => {
      closed = true
      connChangeRef.current?.('disconnected')
      window.clearTimeout(timer)
      source?.close()
    }
  }, [url, token, lastEventId])
}
