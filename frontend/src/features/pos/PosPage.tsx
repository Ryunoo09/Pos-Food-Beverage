import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Minus, Plus, Search, Trash2, Utensils } from 'lucide-react'
import { useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { menuApi } from '../../api/menu'
import type { ModifierGroup, Product } from '../../api/menu'
import {
  PAYMENT_METHODS,
  SERVICE_TYPES,
  SERVICE_TYPE_LABELS,
  orderApi,
  paymentMethodLabel,
} from '../../api/orders'
import type {
  AddItemInput,
  Order,
  PaymentInput,
  PaymentResult,
  Receipt,
  ServiceType,
} from '../../api/orders'
import { shiftApi } from '../../api/shifts'
import { tableApi } from '../../api/tables'
import type { Table } from '../../api/tables'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { apiMessage } from '../../utils/apiError'
import { cn } from '../../utils/cn'
import { formatDate } from '../../utils/formatDate'
import { formatMoney } from '../../utils/formatMoney'
import { generateIdempotencyKey } from '../../utils/idempotency'
import { printReceipt } from '../../utils/printReceipt'

export function PosPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [params] = useSearchParams()

  const [serviceType, setServiceType] = useState<ServiceType>('DINE_IN')
  const [tableID, setTableID] = useState(params.get('table') ?? '')
  const [priority, setPriority] = useState<'NORMAL' | 'RUSH'>('NORMAL')
  const [customerName, setCustomerName] = useState('')
  const [customerPhone, setCustomerPhone] = useState('')
  const [deliveryAddress, setDeliveryAddress] = useState('')
  const [search, setSearch] = useState('')
  const [categoryID, setCategoryID] = useState('all')
  const [order, setOrder] = useState<Order | null>(null)
  const [selected, setSelected] = useState<Product | null>(null)
  const [addError, setAddError] = useState<string | null>(null)
  const [confirmPending, setConfirmPending] = useState(false)
  const [paymentOpen, setPaymentOpen] = useState(false)
  const [paymentKey, setPaymentKey] = useState('')
  const [paymentError, setPaymentError] = useState<string | null>(null)
  const [receipt, setReceipt] = useState<Receipt | null>(null)
  const [lastPayment, setLastPayment] = useState<PaymentResult | null>(null)
  const [openShiftOpen, setOpenShiftOpen] = useState(false)
  const [payAfterShift, setPayAfterShift] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const confirmKey = useRef<string | null>(null)

  const products = useQuery({
    queryKey: ['pos-products'],
    queryFn: async () =>
      (await menuApi.products.list({ is_active: true, per_page: 100 })).data.data,
  })
  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: async () => (await menuApi.categories.list()).data.data,
  })
  const tables = useQuery({
    queryKey: ['tables'],
    queryFn: async () => (await tableApi.list()).data.data,
  })
  const currentShift = useQuery({
    queryKey: ['shift-current'],
    queryFn: async () => (await shiftApi.current()).data.data,
    retry: false,
  })

  const locked = order !== null
  const table = (tables.data ?? []).find((item) => item.id === tableID)
  const visibleProducts = (products.data ?? []).filter((product) => {
    const matchCategory = categoryID === 'all' || product.category_id === categoryID
    const needle = search.trim().toLowerCase()
    const matchSearch =
      needle === '' ||
      product.name.toLowerCase().includes(needle) ||
      product.sku.toLowerCase().includes(needle)
    return matchCategory && matchSearch
  })

  const addItem = useMutation({
    mutationFn: async (input: AddItemInput) => {
      let orderID = order?.id
      if (!orderID) {
        const draft = await orderApi.createDraft({
          service_type: serviceType,
          table_id: serviceType === 'DINE_IN' ? tableID || null : null,
          customer_name: customerName.trim() || null,
          customer_phone: customerPhone.trim() || null,
          delivery_address: serviceType === 'DELIVERY' ? deliveryAddress.trim() || null : null,
          note: null,
          priority,
        })
        orderID = draft.data.data.id
      }
      const result = await orderApi.addItem(orderID, input)
      return result.data.data
    },
    onSuccess: (updated) => {
      setOrder(updated)
      setSelected(null)
      setAddError(null)
    },
    onError: (error) => setAddError(apiMessage(error)),
  })

  const updateItem = useMutation({
    mutationFn: async ({ itemID, quantity }: { itemID: string; quantity: number }) => {
      if (!order) throw new Error('Pesanan belum ada.')
      if (quantity < 1) return (await orderApi.deleteItem(order.id, itemID)).data.data
      return (await orderApi.updateItem(order.id, itemID, { quantity })).data.data
    },
    onSuccess: setOrder,
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const removeItem = useMutation({
    mutationFn: async (itemID: string) => {
      if (!order) throw new Error('Pesanan belum ada.')
      return (await orderApi.deleteItem(order.id, itemID)).data.data
    },
    onSuccess: setOrder,
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const openShift = useMutation({
    mutationFn: (amount: string) => shiftApi.open(amount),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['shift-current'] })
      setOpenShiftOpen(false)
      toast.push('Shift dibuka.')
      if (payAfterShift) {
        setPayAfterShift(false)
        setPaymentKey(generateIdempotencyKey())
        setPaymentOpen(true)
      }
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const pay = useMutation({
    mutationFn: async (input: PaymentInput) => {
      if (!order) throw new Error('Pesanan belum ada.')
      const result = await orderApi.createPayment(order.id, paymentKey, input)
      const receiptResult = await orderApi.receipt(order.id)
      return { result: result.data.data, receipt: receiptResult.data.data }
    },
    onSuccess: ({ result, receipt: printed }) => {
      setPaymentOpen(false)
      setPaymentKey('')
      setPaymentError(null)
      setLastPayment(result)
      setReceipt(printed)
      void queryClient.invalidateQueries({ queryKey: ['tables'] })
      toast.push(
        result.payment.change_amount
          ? `Pembayaran diterima. Kembalian ${formatMoney(result.payment.change_amount)}.`
          : 'Pembayaran diterima.',
      )
    },
    onError: (error) => setPaymentError(apiMessage(error)),
  })

  const cancelDraft = useMutation({
    mutationFn: async () => {
      if (!order) throw new Error('Pesanan belum ada.')
      return orderApi.cancel(order.id, {
        reason_code: 'CANCELLED_BY_CASHIER',
        note: null,
        mode: 'SALE_REVERSAL',
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      setCancelOpen(false)
      resetSale()
      toast.push('Pesanan dibatalkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  async function handleConfirm() {
    if (!order) return
    setConfirmPending(true)
    confirmKey.current ??= generateIdempotencyKey()
    try {
      const result = await orderApi.confirm(order.id, confirmKey.current)
      confirmKey.current = null
      setOrder(result.data.data)
      await queryClient.invalidateQueries({ queryKey: ['tables'] })
      toast.push('Pesanan dikirim ke dapur.')
      if (!currentShift.data) {
        setPayAfterShift(true)
        setOpenShiftOpen(true)
        return
      }
      setPaymentKey(generateIdempotencyKey())
      setPaymentOpen(true)
    } catch (error) {
      toast.push(apiMessage(error), 'error')
    } finally {
      setConfirmPending(false)
    }
  }

  function resetSale() {
    setOrder(null)
    setTableID('')
    setCustomerName('')
    setCustomerPhone('')
    setDeliveryAddress('')
    setPriority('NORMAL')
    setSelected(null)
    setAddError(null)
    setReceipt(null)
    setLastPayment(null)
    setPaymentKey('')
    setPaymentError(null)
    confirmKey.current = null
  }

  function pickProduct(product: Product) {
    if (!product.is_available) {
      toast.push(`${product.name} sedang habis.`, 'error')
      return
    }
    if (locked) {
      setSelected(product)
      return
    }
    if (serviceType === 'DINE_IN' && !tableID) {
      toast.push('Pilih meja dulu sebelum menambah pesanan.', 'error')
      return
    }
    if (serviceType === 'DELIVERY' && (!customerPhone.trim() || !deliveryAddress.trim())) {
      toast.push('Isi nomor telepon dan alamat untuk pesanan delivery.', 'error')
      return
    }
    setAddError(null)
    setSelected(product)
  }

  const items = (order?.items ?? []).filter((item) => item.status === 'ACTIVE')
  const canPay = order !== null && order.status !== 'DRAFT' && order.status !== 'CANCELLED'

  return (
    <div className="grid gap-4 lg:h-[calc(100dvh-6.5rem)] lg:min-h-0 lg:grid-cols-[minmax(0,1fr)_23rem]">
      <section className="flex min-h-0 flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          {SERVICE_TYPES.map((type) => (
            <button
              key={type}
              type="button"
              disabled={locked}
              onClick={() => {
                setServiceType(type)
                if (type !== 'DINE_IN') setTableID('')
              }}
              className={cn(
                'min-h-11 rounded-control border px-4 text-sm font-medium disabled:opacity-50',
                serviceType === type
                  ? 'border-ink bg-ink text-surface'
                  : 'border-control bg-surface text-ink hover:bg-raised',
              )}
            >
              {SERVICE_TYPE_LABELS[type]}
            </button>
          ))}
          <button
            type="button"
            disabled={locked}
            onClick={() => setPriority(priority === 'RUSH' ? 'NORMAL' : 'RUSH')}
            className={cn(
              'min-h-11 rounded-control border px-4 text-sm font-medium disabled:opacity-50',
              priority === 'RUSH'
                ? 'border-warning bg-pill-warning text-pill-warning-ink'
                : 'border-control bg-surface text-muted hover:bg-raised',
            )}
          >
            RUSH
          </button>
          <span className="ml-auto text-xs text-muted">
            {currentShift.data
              ? `Shift aktif sejak ${formatDate(currentShift.data.opened_at)}`
              : 'Shift belum dibuka'}
          </span>
          {!currentShift.data && (
            <Button variant="secondary" size="sm" onClick={() => setOpenShiftOpen(true)}>
              Buka shift
            </Button>
          )}
        </div>

        {serviceType === 'DINE_IN' ? (
          <div className="w-full sm:w-64">
            <Select
              aria-label="Pilih meja"
              placeholder="Pilih meja"
              value={tableID}
              onValueChange={setTableID}
              disabled={locked}
              options={availableTables(tables.data ?? [], tableID).map((item) => ({
                value: item.id,
                label: `${item.number}${item.area ? ` · ${item.area}` : ''}`,
              }))}
            />
          </div>
        ) : (
          <div className="grid gap-2 sm:grid-cols-3">
            <Input
              aria-label="Nama pelanggan"
              placeholder="Nama pelanggan"
              value={customerName}
              disabled={locked}
              onChange={(event) => setCustomerName(event.target.value)}
            />
            <Input
              aria-label="Nomor telepon"
              placeholder="Nomor telepon"
              inputMode="tel"
              value={customerPhone}
              disabled={locked}
              onChange={(event) => setCustomerPhone(event.target.value)}
            />
            {serviceType === 'DELIVERY' && (
              <Input
                aria-label="Alamat pengiriman"
                placeholder="Alamat pengiriman"
                value={deliveryAddress}
                disabled={locked}
                onChange={(event) => setDeliveryAddress(event.target.value)}
              />
            )}
          </div>
        )}

        <div className="flex items-center gap-2">
          <Input
            aria-label="Cari menu"
            placeholder="Cari nama atau SKU…"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            trailing={<Search className="size-4 text-muted" strokeWidth={1.5} />}
          />
        </div>

        <div className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
          <CategoryChip active={categoryID === 'all'} onClick={() => setCategoryID('all')}>
            Semua
          </CategoryChip>
          {(categories.data ?? [])
            .filter((category) => category.is_active)
            .map((category) => (
              <CategoryChip
                key={category.id}
                active={categoryID === category.id}
                onClick={() => setCategoryID(category.id)}
              >
                {category.name}
              </CategoryChip>
            ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {products.isLoading ? (
            <div className="grid place-items-center py-16">
              <Spinner label="Memuat menu" />
            </div>
          ) : products.isError ? (
            <p className="py-8 text-sm text-negative">{apiMessage(products.error)}</p>
          ) : visibleProducts.length === 0 ? (
            <p className="py-8 text-sm text-muted">Tidak ada menu yang cocok.</p>
          ) : (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
              {visibleProducts.map((product) => (
                <button
                  key={product.id}
                  type="button"
                  onClick={() => pickProduct(product)}
                  className={cn(
                    'flex min-h-28 flex-col items-start justify-between rounded-card border border-line bg-surface p-3 text-left',
                    product.is_available
                      ? 'hover:border-brand-600'
                      : 'cursor-not-allowed opacity-55',
                  )}
                >
                  <span className="flex w-full items-start justify-between gap-2">
                    <span className="text-sm leading-snug font-medium">{product.name}</span>
                    {!product.is_available && <Badge tone="red">Habis</Badge>}
                  </span>
                  <span className="num mt-2 text-sm text-muted">{formatMoney(product.price)}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </section>

      <aside className="flex min-h-0 flex-col rounded-card border border-line bg-surface">
        <header className="flex items-center justify-between gap-2 border-b border-rule px-4 py-3">
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold">
              {order?.order_no ?? (order ? 'Draft pesanan' : 'Pesanan baru')}
            </p>
            <p className="text-xs text-muted">
              {SERVICE_TYPE_LABELS[serviceType]}
              {table ? ` · Meja ${table.number}` : ''}
              {priority === 'RUSH' ? ' · RUSH' : ''}
            </p>
          </div>
          {order && (
            <Button
              variant="ghost"
              size="sm"
              className="text-negative"
              onClick={() => setCancelOpen(true)}
              aria-label="Batalkan pesanan"
            >
              <Trash2 className="size-4" strokeWidth={1.5} />
            </Button>
          )}
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
          {items.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted">
              Pilih menu di kiri untuk mulai pesanan.
            </p>
          ) : (
            <ul className="divide-y divide-rule">
              {items.map((item) => (
                <li key={item.id} className="py-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{item.product_name}</p>
                      <p className="text-xs text-muted">{item.variant_name}</p>
                      {item.modifiers && item.modifiers.length > 0 && (
                        <p className="mt-0.5 text-xs text-muted">
                          {item.modifiers.map((modifier) => modifier.option_name).join(', ')}
                        </p>
                      )}
                      {item.note && <p className="mt-0.5 text-xs text-muted">Catatan: {item.note}</p>}
                    </div>
                    <span className="num text-sm">{formatMoney(item.line_total)}</span>
                  </div>
                  <div className="mt-2 flex items-center gap-1">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() =>
                        updateItem.mutate({ itemID: item.id, quantity: item.quantity - 1 })
                      }
                      disabled={updateItem.isPending}
                      aria-label={`Kurangi ${item.product_name}`}
                    >
                      <Minus className="size-4" strokeWidth={2} />
                    </Button>
                    <span className="num w-9 text-center text-sm font-medium">{item.quantity}</span>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() =>
                        updateItem.mutate({ itemID: item.id, quantity: item.quantity + 1 })
                      }
                      disabled={updateItem.isPending}
                      aria-label={`Tambah ${item.product_name}`}
                    >
                      <Plus className="size-4" strokeWidth={2} />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="ml-auto text-negative"
                      onClick={() => removeItem.mutate(item.id)}
                      disabled={removeItem.isPending}
                      aria-label={`Hapus ${item.product_name}`}
                    >
                      <Trash2 className="size-4" strokeWidth={1.5} />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        <footer className="space-y-2 border-t border-rule px-4 py-3">
          <TotalRow label="Subtotal" value={order?.subtotal_amount} />
          <TotalRow label="Diskon" value={order?.item_discount_amount} negative />
          <TotalRow label="Service" value={order?.service_charge_amount} />
          <TotalRow label="Pajak" value={order?.tax_amount} />
          <TotalRow label="Pembulatan" value={order?.rounding_amount} />
          <div className="flex items-center justify-between border-t border-rule pt-2">
            <span className="text-sm font-semibold">Total</span>
            <span className="num text-lg font-semibold">
              {formatMoney(order?.total_amount ?? '0')}
            </span>
          </div>
          {canPay ? (
            <Button
              className="w-full"
              size="lg"
              onClick={() => {
                setPaymentKey(generateIdempotencyKey())
                setPaymentError(null)
                setPaymentOpen(true)
              }}
            >
              Bayar
            </Button>
          ) : (
            <Button
              className="w-full"
              size="lg"
              disabled={items.length === 0}
              loading={confirmPending}
              onClick={handleConfirm}
            >
              <Utensils className="size-4" strokeWidth={2} />
              Kirim ke dapur
            </Button>
          )}
        </footer>
      </aside>

      {selected && (
        <ProductDialog
          product={selected}
          open={selected !== null}
          adding={addItem.isPending}
          error={addError}
          onOpenChange={(open) => {
            if (!open && !addItem.isPending) {
              setSelected(null)
              setAddError(null)
            }
          }}
          onAdd={(input) => addItem.mutate(input)}
        />
      )}

      <OpenShiftDialog
        open={openShiftOpen}
        loading={openShift.isPending}
        onOpenChange={(open) => {
          setOpenShiftOpen(open)
          if (!open) setPayAfterShift(false)
        }}
        onSubmit={(amount) => openShift.mutate(amount)}
      />

      {order && (
        <Dialog
          open={paymentOpen}
          onOpenChange={(open) => {
            if (!open && !pay.isPending) {
              setPaymentOpen(false)
              setPaymentError(null)
            }
          }}
          title="Pembayaran"
          description={`Tagihan ${order.order_no ?? 'draft'}`}
          footer={
            <>
              <Button
                variant="secondary"
                onClick={() => setPaymentOpen(false)}
                disabled={pay.isPending}
              >
                Nanti
              </Button>
              <Button type="submit" form="payment-form" loading={pay.isPending}>
                Terima pembayaran
              </Button>
            </>
          }
        >
          <PaymentForm
            order={order}
            error={paymentError}
            onSubmit={(input) => pay.mutate(input)}
          />
        </Dialog>
      )}

      <ReceiptDialog
        receipt={receipt}
        changeAmount={lastPayment?.payment.change_amount}
        onClose={resetSale}
      />

      <Dialog
        open={cancelOpen}
        onOpenChange={(open) => !cancelDraft.isPending && setCancelOpen(open)}
        title="Batalkan pesanan?"
        description="Item draft akan dihapus dan stok tidak berubah karena pesanan belum dikirim ke dapur."
        footer={
          <>
            <Button variant="secondary" onClick={() => setCancelOpen(false)} disabled={cancelDraft.isPending}>
              Kembali
            </Button>
            <Button variant="danger" loading={cancelDraft.isPending} onClick={() => cancelDraft.mutate()}>
              Batalkan pesanan
            </Button>
          </>
        }
      />
    </div>
  )
}

function availableTables(tables: Table[], selectedID: string): Table[] {
  return tables.filter(
    (table) => table.is_active && (table.status === 'AVAILABLE' || table.id === selectedID),
  )
}

function CategoryChip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'min-h-9 shrink-0 rounded-chip border px-3 text-sm whitespace-nowrap',
        active ? 'border-ink bg-ink text-surface' : 'border-line bg-surface text-muted hover:bg-raised',
      )}
    >
      {children}
    </button>
  )
}

function TotalRow({
  label,
  value,
  negative = false,
}: {
  label: string
  value?: string
  negative?: boolean
}) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted">{label}</span>
      <span className="num">
        {negative && value !== '0.00' ? '-' : ''}
        {formatMoney(value ?? '0')}
      </span>
    </div>
  )
}

function PaymentForm({
  order,
  error,
  onSubmit,
}: {
  order: Order
  error: string | null
  onSubmit: (input: PaymentInput) => void
}) {
  const [method, setMethod] = useState('CASH')
  const [amount, setAmount] = useState(order.total_amount)
  const [cashReceived, setCashReceived] = useState('')
  const [reference, setReference] = useState('')
  const meta = PAYMENT_METHODS.find((item) => item.code === method) ?? PAYMENT_METHODS[0]
  const validAmount = /^\d+(\.\d+)?$/.test(amount.trim()) && Number(amount) > 0

  return (
    <form
      id="payment-form"
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        if (!validAmount) return
        onSubmit({
          method_code: method,
          amount: amount.trim(),
          cash_received: meta.isCash ? cashReceived.trim() || null : null,
          reference_no: meta.requiresReference ? reference.trim() || null : null,
        })
      }}
    >
      <div className="grid grid-cols-2 gap-2">
        {PAYMENT_METHODS.map((item) => (
          <button
            key={item.code}
            type="button"
            onClick={() => setMethod(item.code)}
            className={cn(
              'min-h-11 rounded-control border px-3 text-sm font-medium',
              method === item.code
                ? 'border-ink bg-ink text-surface'
                : 'border-control bg-surface hover:bg-raised',
            )}
          >
            {item.label}
          </button>
        ))}
      </div>
      <Input
        label="Jumlah bayar"
        inputMode="decimal"
        value={amount}
        onChange={(event) => setAmount(event.target.value)}
        helper={`Total tagihan ${formatMoney(order.total_amount)}`}
      />
      {meta.isCash && (
        <Input
          label="Uang diterima"
          inputMode="decimal"
          placeholder="Kosongkan bila uang pas"
          value={cashReceived}
          onChange={(event) => setCashReceived(event.target.value)}
        />
      )}
      {meta.requiresReference && (
        <Input
          label="Nomor referensi"
          placeholder="Nomor transaksi / approval"
          value={reference}
          onChange={(event) => setReference(event.target.value)}
        />
      )}
      <p className="text-xs text-muted">Metode: {paymentMethodLabel(method)}</p>
      {error && (
        <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
          {error}
        </p>
      )}
    </form>
  )
}

function OpenShiftDialog({
  open,
  loading,
  onOpenChange,
  onSubmit,
}: {
  open: boolean
  loading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (amount: string) => void
}) {
  const [amount, setAmount] = useState('0')
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Buka shift"
      description="Pembayaran hanya bisa dilakukan saat shift terbuka. Masukkan modal kas awal."
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button onClick={() => onSubmit(amount.trim() || '0')} loading={loading}>
            Buka shift
          </Button>
        </>
      }
    >
      <Input
        label="Modal awal"
        inputMode="decimal"
        value={amount}
        onChange={(event) => setAmount(event.target.value)}
        helper="Boleh 0 bila kas awal kosong."
      />
    </Dialog>
  )
}

function ReceiptDialog({
  receipt,
  changeAmount,
  onClose,
}: {
  receipt: Receipt | null
  changeAmount?: string
  onClose: () => void
}) {
  return (
    <Dialog
      open={receipt !== null}
      onOpenChange={(open) => !open && onClose()}
      title="Struk"
      description={receipt?.order.order_no ?? ''}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Selesai
          </Button>
          <Button onClick={() => receipt && printReceipt(receipt, changeAmount)}>Cetak struk</Button>
        </>
      }
    >
      {receipt && <ReceiptPreview receipt={receipt} changeAmount={changeAmount} />}
    </Dialog>
  )
}

export function ReceiptPreview({
  receipt,
  changeAmount,
}: {
  receipt: Receipt
  changeAmount?: string
}) {
  return (
    <div className="space-y-3 text-sm">
      <div>
        <p className="font-semibold">{receipt.outlet.name}</p>
        {receipt.outlet.address && <p className="text-xs text-muted">{receipt.outlet.address}</p>}
      </div>
      <div className="flex justify-between text-xs text-muted">
        <span>{receipt.order.order_no}</span>
        <span>{formatDate(receipt.order.created_at)}</span>
      </div>
      <ul className="divide-y divide-rule border-y border-rule">
        {(receipt.order.items ?? [])
          .filter((item) => item.status !== 'VOID')
          .map((item) => (
            <li key={item.id} className="flex justify-between gap-2 py-2">
              <span className="min-w-0">
                <span className="font-medium">{item.product_name}</span>
                <span className="block text-xs text-muted">
                  {item.variant_name} · {item.quantity} x {formatMoney(item.unit_price)}
                </span>
              </span>
              <span className="num shrink-0">{formatMoney(item.line_total)}</span>
            </li>
          ))}
      </ul>
      <div className="space-y-1">
        <TotalRow label="Subtotal" value={receipt.order.subtotal_amount} />
        <TotalRow label="Pajak" value={receipt.order.tax_amount} />
        <TotalRow label="Service" value={receipt.order.service_charge_amount} />
        <TotalRow label="Pembulatan" value={receipt.order.rounding_amount} />
        <div className="flex justify-between border-t border-rule pt-1 font-semibold">
          <span>Total</span>
          <span className="num">{formatMoney(receipt.order.total_amount)}</span>
        </div>
        {(receipt.payments ?? []).map((payment) => (
          <div key={payment.id} className="flex justify-between text-xs text-muted">
            <span>{paymentMethodLabel(payment.method_code)}</span>
            <span className="num">{formatMoney(payment.amount)}</span>
          </div>
        ))}
        {changeAmount && (
          <div className="flex justify-between text-xs">
            <span>Kembalian</span>
            <span className="num">{formatMoney(changeAmount)}</span>
          </div>
        )}
      </div>
    </div>
  )
}

function ProductDialog({
  product,
  open,
  adding,
  error,
  onOpenChange,
  onAdd,
}: {
  product: Product
  open: boolean
  adding: boolean
  error: string | null
  onOpenChange: (open: boolean) => void
  onAdd: (input: AddItemInput) => void
}) {
  const [quantity, setQuantity] = useState(1)
  const [note, setNote] = useState('')
  const [variantID, setVariantID] = useState('')
  const [options, setOptions] = useState<Record<string, string[]>>({})

  const detail = useQuery({
    queryKey: ['product', product.id],
    queryFn: async () => (await menuApi.products.get(product.id)).data.data,
  })
  const groups = useQuery({
    queryKey: ['modifier-groups'],
    queryFn: async () => (await menuApi.modifiers.list()).data.data,
  })

  const variants = (detail.data?.variants ?? []).filter((variant) => variant.is_active)
  const defaultVariant = variants.find((variant) => variant.is_default) ?? variants[0]
  const activeVariantID = variants.some((variant) => variant.id === variantID)
    ? variantID
    : (defaultVariant?.id ?? '')
  const activeVariant = variants.find((variant) => variant.id === activeVariantID)
  const assignedGroups = (detail.data?.modifier_group_ids ?? [])
    .map((id) => (groups.data ?? []).find((group) => group.id === id))
    .filter((group): group is ModifierGroup => Boolean(group?.is_active))

  function selectedCount(group: ModifierGroup): number {
    return (options[group.id] ?? []).length
  }

  function toggleOption(group: ModifierGroup, optionID: string) {
    setOptions((current) => {
      const picked = current[group.id] ?? []
      const next = picked.includes(optionID)
        ? picked.filter((id) => id !== optionID)
        : group.max_select === 1
          ? [optionID]
          : [...picked, optionID].slice(0, group.max_select)
      return { ...current, [group.id]: next }
    })
  }

  const invalidGroup = assignedGroups.find((group) => {
    const count = selectedCount(group)
    return count < group.min_select || count > group.max_select
  })
  const ready = activeVariantID !== '' && !invalidGroup && !detail.isLoading

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => !adding && onOpenChange(next)}
      title={product.name}
      description={detail.data?.description || product.sku}
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={adding}>
            Batal
          </Button>
          <Button
            loading={adding}
            disabled={!ready}
            onClick={() =>
              onAdd({
                product_id: product.id,
                variant_id: activeVariantID,
                quantity,
                modifier_option_ids: assignedGroups.flatMap((group) => options[group.id] ?? []),
                note: note.trim() || null,
              })
            }
          >
            Tambah · {formatMoney(activeVariant?.price ?? product.price)}
          </Button>
        </>
      }
    >
      {detail.isLoading ? (
        <Spinner label="Memuat detail menu" />
      ) : detail.isError ? (
        <p className="text-sm text-negative">{apiMessage(detail.error)}</p>
      ) : (
        <div className="space-y-4">
          {variants.length > 1 && (
            <fieldset>
              <legend className="mb-2 text-sm font-medium">Varian</legend>
              <div className="flex flex-wrap gap-2">
                {variants.map((variant) => (
                  <button
                    key={variant.id}
                    type="button"
                    onClick={() => setVariantID(variant.id)}
                    className={cn(
                      'min-h-11 rounded-control border px-3 text-sm',
                      activeVariantID === variant.id
                        ? 'border-ink bg-ink text-surface'
                        : 'border-control bg-surface hover:bg-raised',
                    )}
                  >
                    {variant.name} · {formatMoney(variant.price)}
                  </button>
                ))}
              </div>
            </fieldset>
          )}

          {assignedGroups.map((group) => {
            const count = selectedCount(group)
            const invalid = count < group.min_select || count > group.max_select
            return (
              <fieldset key={group.id}>
                <legend className="mb-2 flex items-center gap-2 text-sm font-medium">
                  {group.name}
                  {group.is_required && <Badge tone="blue">Wajib</Badge>}
                  <span className="text-xs font-normal text-muted">
                    {group.min_select}–{group.max_select}
                  </span>
                </legend>
                <div className="space-y-1">
                  {group.options
                    .filter((option) => option.is_active)
                    .map((option) => {
                      const picked = (options[group.id] ?? []).includes(option.id)
                      return (
                        <button
                          key={option.id}
                          type="button"
                          onClick={() => toggleOption(group, option.id)}
                          className={cn(
                            'flex min-h-11 w-full items-center justify-between gap-3 rounded-control border px-3 text-sm',
                            picked ? 'border-brand-600 bg-raised' : 'border-control hover:bg-raised',
                          )}
                        >
                          <span>{option.name}</span>
                          <span className="num text-xs text-muted">
                            {Number(option.price_delta) === 0 ? '' : `+${formatMoney(option.price_delta)}`}
                          </span>
                        </button>
                      )
                    })}
                </div>
                {invalid && (
                  <p className="mt-1 text-xs text-negative">
                    Pilih {group.min_select}–{group.max_select} opsi.
                  </p>
                )}
              </fieldset>
            )
          })}

          <div className="flex items-end gap-2">
            <div>
              <p className="mb-2 text-sm font-medium">Jumlah</p>
              <div className="flex items-center gap-1">
                <Button
                  variant="secondary"
                  onClick={() => setQuantity((value) => Math.max(1, value - 1))}
                  aria-label="Kurangi jumlah"
                >
                  <Minus className="size-4" strokeWidth={2} />
                </Button>
                <span className="num w-10 text-center text-base font-semibold">{quantity}</span>
                <Button
                  variant="secondary"
                  onClick={() => setQuantity((value) => Math.min(99, value + 1))}
                  aria-label="Tambah jumlah"
                >
                  <Plus className="size-4" strokeWidth={2} />
                </Button>
              </div>
            </div>
            <div className="flex-1">
              <Input
                label="Catatan"
                placeholder="Contoh: tanpa sambal"
                value={note}
                onChange={(event) => setNote(event.target.value)}
              />
            </div>
          </div>

          {error && (
            <p role="alert" className="rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
              {error}
            </p>
          )}
        </div>
      )}
    </Dialog>
  )
}
