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
