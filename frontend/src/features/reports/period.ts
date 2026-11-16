import { useSearchParams } from 'react-router-dom'

export function todayISO(): string {
  const now = new Date()
  return new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
}

/** Awal bulan berjalan, dipakai sebagai nilai default rentang laporan. */
export function monthStartISO(): string {
  const now = new Date()
  return new Date(now.getFullYear(), now.getMonth(), 1).toLocaleDateString('sv-SE').slice(0, 10)
}

/**
 * Periode laporan disimpan di query string supaya bisa dibagikan dan tetap
 * sama saat halaman di-refresh.
 */
export function useReportPeriod() {
  const [params, setParams] = useSearchParams()
  const from = params.get('from') || monthStartISO()
  const to = params.get('to') || todayISO()

  function setPeriod(next: { from: string; to: string }) {
    const search = new URLSearchParams(params)
    search.set('from', next.from)
    search.set('to', next.to)
    setParams(search, { replace: true })
  }

  return { from, to, setPeriod }
}
