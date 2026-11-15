/**
 * Data contoh untuk layar ringkasan. Fase 6 mengganti modul ini dengan
 * respons API (`GET /api/v1/reports/sales` + `GET /api/v1/orders`).
 * Semua angka saling terikat: total = jumlah baris, rata-rata = total / pesanan.
 */
export interface HourBucket {
  hour: number
  sales: number
  orders: number
}

export interface RecentOrder {
  code: string
  place: string
  items: number
  total: number
  status: 'Dikonfirmasi' | 'Dimasak' | 'Siap' | 'Selesai'
}

export const today = {
  date: 'Minggu, 15 November 2026',
  openedAt: '08.00',
  hours: [
    { hour: 8, sales: 180_000, orders: 3 },
    { hour: 9, sales: 240_000, orders: 4 },
    { hour: 10, sales: 315_000, orders: 5 },
    { hour: 11, sales: 420_000, orders: 6 },
    { hour: 12, sales: 860_000, orders: 12 },
    { hour: 13, sales: 745_000, orders: 10 },
    { hour: 14, sales: 380_000, orders: 6 },
    { hour: 15, sales: 260_000, orders: 4 },
    { hour: 16, sales: 340_000, orders: 5 },
    { hour: 17, sales: 520_000, orders: 7 },
    { hour: 18, sales: 780_000, orders: 11 },
    { hour: 19, sales: 920_000, orders: 13 },
    { hour: 20, sales: 640_000, orders: 9 },
    { hour: 21, sales: 300_000, orders: 5 },
  ] satisfies HourBucket[],
}

export const totalSales = today.hours.reduce((sum, bucket) => sum + bucket.sales, 0)
export const totalOrders = today.hours.reduce((sum, bucket) => sum + bucket.orders, 0)
export const averageOrder = totalSales / totalOrders

export const lastWeek = { sales: 6_120_000, orders: 92 }

export const week = [
  { day: 'Sen', sales: 5_240_000 },
  { day: 'Sel', sales: 4_980_000 },
  { day: 'Rab', sales: 5_610_000 },
  { day: 'Kam', sales: 6_040_000 },
  { day: 'Jum', sales: 7_320_000 },
  { day: 'Sab', sales: 8_150_000 },
  { day: 'Min', sales: totalSales },
]

export const recentOrders: RecentOrder[] = [
  { code: 'ORD-1042', place: 'Meja 4', items: 3, total: 118_000, status: 'Dikonfirmasi' },
  { code: 'ORD-1041', place: 'Bawa pulang', items: 2, total: 46_000, status: 'Siap' },
  { code: 'ORD-1040', place: 'Meja 2', items: 5, total: 214_000, status: 'Dimasak' },
  { code: 'ORD-1039', place: 'GoFood', items: 1, total: 27_000, status: 'Selesai' },
  { code: 'ORD-1038', place: 'Meja 7', items: 4, total: 156_000, status: 'Selesai' },
]

export interface LowStockItem {
  name: string
  left: number
  unit: string
  minimum: number
}

export const lowStock: LowStockItem[] = [
  { name: 'Susu UHT', left: 2.4, unit: 'L', minimum: 6 },
  { name: 'Biji kopi arabika', left: 1.2, unit: 'kg', minimum: 3 },
  { name: 'Beras', left: 8, unit: 'kg', minimum: 10 },
  { name: 'Gula pasir', left: 4.5, unit: 'kg', minimum: 5 },
  { name: 'Ayam fillet', left: 3, unit: 'kg', minimum: 4 },
]


console.assert(totalSales === 6_900_000, 'total penjualan tidak sama dengan jumlah per jam')
console.assert(totalOrders === 100, 'total pesanan tidak sama dengan jumlah per jam')
console.assert(averageOrder === 69_000, 'rata-rata per pesanan tidak sama dengan total / pesanan')
console.assert(week[6]?.sales === totalSales, 'baris hari ini di tabel 7 hari tidak sama dengan total')
