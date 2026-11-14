// Menerima string NUMERIC dari API. parseFloat hanya untuk display, bukan kalkulasi.
export function formatMoney(amount: string): string {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
  }).format(parseFloat(amount))
}
