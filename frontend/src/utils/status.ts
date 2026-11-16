export type StatusTone = 'green' | 'yellow' | 'red' | 'gray' | 'blue'

const ORDER_TONES: Record<string, StatusTone> = {
  DRAFT: 'gray',
  CONFIRMED: 'blue',
  IN_PROGRESS: 'yellow',
  READY: 'green',
  SERVED: 'blue',
  COMPLETED: 'green',
  CANCELLED: 'red',
}

const PAYMENT_TONES: Record<string, StatusTone> = {
  UNPAID: 'red',
  PARTIAL: 'yellow',
  PAID: 'green',
  REFUNDED: 'gray',
  VOID: 'gray',
}

export function orderStatusTone(status: string): StatusTone {
  return ORDER_TONES[status] ?? 'gray'
}

export function paymentStatusTone(status: string): StatusTone {
  return PAYMENT_TONES[status] ?? 'gray'
}

const ORDER_LABELS: Record<string, string> = {
  DRAFT: 'Draft',
  CONFIRMED: 'Dikonfirmasi',
  IN_PROGRESS: 'Diproses',
  READY: 'Siap',
  SERVED: 'Disajikan',
  COMPLETED: 'Selesai',
  CANCELLED: 'Dibatalkan',
}

const PAYMENT_LABELS: Record<string, string> = {
  UNPAID: 'Belum bayar',
  PARTIAL: 'Sebagian',
  PARTIALLY_PAID: 'Sebagian',
  PAID: 'Lunas',
  REFUNDED: 'Dikembalikan',
  PARTIALLY_REFUNDED: 'Refund sebagian',
  VOID: 'Void',
}

export function orderStatusLabel(status: string): string {
  return ORDER_LABELS[status] ?? status
}

export function paymentStatusLabel(status: string): string {
  return PAYMENT_LABELS[status] ?? status
}
