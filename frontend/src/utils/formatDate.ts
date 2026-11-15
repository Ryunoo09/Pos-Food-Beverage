/** Format tanggal ISO ke waktu lokal Indonesia. Zona bisa dioverride per outlet. */
export function formatDate(iso: string, timeZone = 'Asia/Jakarta'): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone,
  }).format(date)
}
