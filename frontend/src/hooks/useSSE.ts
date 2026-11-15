import { useEffect, useRef } from 'react'

export interface SSEEvent {
  id: string
  type: string
  data: unknown
}

const MAX_DELAY = 15_000

/**
 * EventSource dengan backoff 1, 2, 4, 8, 15 detik dan Last-Event-ID saat
 * reconnect.
 *
 * ponytail: kontrak auth stream belum ada (backend SSE menyusul di fase
 * realtime). EventSource tidak bisa mengirim header Authorization dan cookie
 * refresh dibatasi Path=/api/v1/auth, jadi endpoint stream wajib menerima
 * tiket/token lewat query atau cookie khusus. Hanya event `message` yang
 * ditangani; tambahkan addEventListener saat backend memakai event bernama.
 */
export function useSSE(url: string | null, onEvent: (event: SSEEvent) => void) {
  const handlerRef = useRef(onEvent)

  useEffect(() => {
    handlerRef.current = onEvent
  })

  useEffect(() => {
    if (!url) return

    let source: EventSource | null = null
    let lastEventId = ''
    let attempt = 0
    let timer: number | undefined
    let closed = false

    function connect() {
      const target = new URL(url!, window.location.origin)
      if (lastEventId) target.searchParams.set('last_event_id', lastEventId)

      source = new EventSource(target, { withCredentials: true })

      source.onopen = () => {
        attempt = 0
      }

      source.onmessage = (event) => {
        if (event.lastEventId) lastEventId = event.lastEventId
        handlerRef.current({ id: event.lastEventId, type: event.type, data: parseData(event.data) })
      }

      source.onerror = () => {
        source?.close()
        if (closed) return
        const delay = Math.min(MAX_DELAY, 1000 * 2 ** attempt)
        attempt += 1
        timer = window.setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      closed = true
      window.clearTimeout(timer)
      source?.close()
    }
  }, [url])
}

function parseData(raw: string): unknown {
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}
