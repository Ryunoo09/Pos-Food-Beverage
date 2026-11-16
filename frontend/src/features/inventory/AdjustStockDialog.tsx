import { useMemo, useState } from 'react'
import { ADJUSTMENT_REASONS } from '../../api/stock'
import type { StockLevel } from '../../api/stock'
import { Button } from '../../components/ui/Button'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'

export interface AdjustStockPayload {
  delta: string
  reason: string
  note: string
}

export function AdjustStockDialog({
  stock,
  open,
  loading,
  onOpenChange,
  onSubmit,
}: {
  stock: StockLevel
  open: boolean
  loading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: AdjustStockPayload) => void
}) {
  const [direction, setDirection] = useState<'IN' | 'OUT'>('IN')
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState<string>(ADJUSTMENT_REASONS[0].value)
  const [note, setNote] = useState('')

  // Hanya tampilan: perubahan saldo dihitung server dari quantity_delta.
  const projected = useMemo(() => {
    const base = Number(stock.quantity_on_hand)
    const value = Number(amount || '0')
    if (Number.isNaN(base) || Number.isNaN(value)) return null
    return direction === 'IN' ? base + value : base - value
  }, [amount, direction, stock.quantity_on_hand])

  const amountValid = /^\d+(\.\d{1,3})?$/.test(amount.trim()) && Number(amount) > 0

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title={`Sesuaikan stok ${stock.name}`}
      description={`Saldo sekarang ${stock.quantity_on_hand} ${stock.base_unit_code}.`}
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button
            loading={loading}
            disabled={!amountValid || note.trim() === ''}
            onClick={() =>
              onSubmit({
                delta: direction === 'IN' ? amount.trim() : `-${amount.trim()}`,
                reason,
                note,
              })
            }
          >
            Simpan penyesuaian
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Select
          label="Arah"
          value={direction}
          onValueChange={(value) => setDirection(value as 'IN' | 'OUT')}
          options={[
            { value: 'IN', label: 'Tambah stok' },
            { value: 'OUT', label: 'Kurangi stok' },
          ]}
        />
        <Input
          label={`Jumlah (${stock.base_unit_code})`}
          inputMode="decimal"
          value={amount}
          onChange={(event) => setAmount(event.target.value)}
          helper={
            projected !== null
              ? `Saldo setelah penyesuaian ${formatQuantity(projected)} ${stock.base_unit_code}`
              : undefined
          }
        />
        <Select
          label="Alasan"
          value={reason}
          onValueChange={setReason}
          options={ADJUSTMENT_REASONS.map((item) => ({ value: item.value, label: item.label }))}
        />
        <Input
          label="Catatan"
          placeholder="Contoh: tumpah saat persiapan"
          value={note}
          onChange={(event) => setNote(event.target.value)}
          helper="Wajib diisi agar jejak audit jelas."
        />
      </div>
    </Dialog>
  )
}

function formatQuantity(value: number): string {
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 3 }).format(value)
}
