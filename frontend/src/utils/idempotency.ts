// Key dibuat sekali sebelum submit dan dipakai ulang saat retry, agar backend
// mengembalikan respons lama alih-alih membuat transaksi ganda.
export function generateIdempotencyKey(): string {
  return crypto.randomUUID()
}
