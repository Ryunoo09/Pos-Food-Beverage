import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import {
  DISCOUNT_SCOPES,
  DISCOUNT_TYPES,
  ROUNDING_MODES,
  SERVICE_TYPES,
  TAX_TYPES,
  settingsApi,
} from '../../api/settings'
import type { Discount, DiscountInput, OutletSettings } from '../../api/settings'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardHeader } from '../../components/ui/Card'
import { Dialog } from '../../components/ui/Dialog'
import { Input } from '../../components/ui/Input'
import { NumberInput } from '../../components/ui/NumberInput'
import { Select } from '../../components/ui/Select'
import { Skeleton } from '../../components/ui/Skeleton'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

export function TaxSettingsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canManageTax = permissions.includes('tax.manage')
  const canManageDiscounts = permissions.includes('discounts.manage')
  const canManageOutlet = permissions.includes('settings.manage')

  const [taxOpen, setTaxOpen] = useState(false)
  const [discountOpen, setDiscountOpen] = useState<Discount | null>(null)

  const outlet = useQuery({
    queryKey: ['settings-outlet'],
    queryFn: async () => (await settingsApi.outlet.get()).data.data,
  })
  const taxes = useQuery({
    queryKey: ['settings-taxes'],
    queryFn: async () => (await settingsApi.taxes.list()).data.data,
  })
  const discounts = useQuery({
    queryKey: ['discounts'],
    queryFn: async () => (await settingsApi.discounts.list()).data.data,
  })

  const createTax = useMutation({
    mutationFn: (body: Parameters<typeof settingsApi.taxes.create>[0]) =>
      settingsApi.taxes.create(body),
    onSuccess: async () => {
      setTaxOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['settings-taxes'] })
      toast.push('Pajak ditambahkan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const toggleTax = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      active ? settingsApi.taxes.activate(id) : settingsApi.taxes.deactivate(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['settings-taxes'] })
      toast.push('Pajak diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const saveDiscount = useMutation({
    mutationFn: ({ id, body }: { id?: string; body: DiscountInput }) =>
      id ? settingsApi.discounts.update(id, body) : settingsApi.discounts.create(body),
    onSuccess: async () => {
      setDiscountOpen(null)
      await queryClient.invalidateQueries({ queryKey: ['discounts'] })
      toast.push('Diskon disimpan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const toggleDiscount = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      active ? settingsApi.discounts.activate(id) : settingsApi.discounts.deactivate(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['discounts'] })
      toast.push('Diskon diperbarui.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  return (
    <>
      <PageHeader
        title="Pajak & diskon"
        description="Tarif pajak, service charge, pembulatan, dan promo yang berlaku di outlet ini."
      />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-12">
        <OutletRulesCard
          settings={outlet.data}
          loading={outlet.isLoading}
          canEdit={canManageOutlet}
          onSave={(body) => settingsApi.outlet.update(body)}
          onSaved={() => queryClient.invalidateQueries({ queryKey: ['settings-outlet'] })}
          onError={(error) => toast.push(apiMessage(error), 'error')}
          onSuccess={() => toast.push('Pengaturan outlet disimpan.')}
        />

        <Card className="lg:col-span-7">
          <CardHeader
            title="Tarif pajak & service"
            action={
              canManageTax && (
                <Button size="sm" variant="secondary" onClick={() => setTaxOpen(true)}>
                  <Plus className="size-4" strokeWidth={2} />
                  Tambah
                </Button>
              )
            }
          />
          {taxes.isLoading ? (
            <div className="space-y-3 p-5">
              {Array.from({ length: 3 }, (_, index) => (
                <Skeleton key={index} className="h-10 w-full" />
              ))}
            </div>
          ) : taxes.isError ? (
            <p className="px-5 py-8 text-sm text-negative">{apiMessage(taxes.error)}</p>
          ) : (taxes.data ?? []).length === 0 ? (
            <p className="px-5 py-10 text-center text-sm text-muted">Belum ada tarif pajak.</p>
          ) : (
            <ul className="divide-y divide-rule">
              {(taxes.data ?? []).map((tax) => (
                <li key={tax.id} className="flex flex-wrap items-center gap-3 px-5 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium">{tax.name}</p>
                      <Badge tone={tax.type === 'TAX' ? 'blue' : 'gray'}>
                        {tax.type === 'TAX' ? 'Pajak' : 'Service'}
                      </Badge>
                      {!tax.is_active && <Badge tone="gray">Nonaktif</Badge>}
                    </div>
                    <p className="mt-0.5 text-xs text-muted">
                      {tax.applies_to.length > 0
                        ? tax.applies_to
                            .map(
                              (code) =>
                                SERVICE_TYPES.find((item) => item.value === code)?.label ?? code,
                            )
                            .join(' · ')
                        : 'Semua jenis layanan'}
                      {' · berlaku sejak '}
                      {formatISODate(tax.effective_from)}
                    </p>
                  </div>
                  <span className="num text-sm tabular-nums">
                    {Number(tax.rate_percent).toLocaleString('id-ID', {
                      maximumFractionDigits: 2,
                    })}
                    %
                  </span>
                  {canManageTax && (
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={toggleTax.isPending}
                      onClick={() => toggleTax.mutate({ id: tax.id, active: !tax.is_active })}
                    >
                      {tax.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="lg:col-span-12">
          <CardHeader
            title="Promo & diskon"
            action={
              canManageDiscounts && (
                <Button size="sm" variant="secondary" onClick={() => setDiscountOpen(emptyDiscount())}>
                  <Plus className="size-4" strokeWidth={2} />
                  Tambah promo
                </Button>
              )
            }
          />
          {discounts.isLoading ? (
            <div className="space-y-3 p-5">
              {Array.from({ length: 3 }, (_, index) => (
                <Skeleton key={index} className="h-10 w-full" />
              ))}
            </div>
          ) : (discounts.data ?? []).length === 0 ? (
            <p className="px-5 py-10 text-center text-sm text-muted">Belum ada promo.</p>
          ) : (
            <ul className="divide-y divide-rule">
              {(discounts.data ?? []).map((discount) => (
                <li key={discount.id} className="flex flex-wrap items-center gap-3 px-5 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="num text-[13px] text-ink">{discount.code}</span>
                      <p className="text-[13px]">{discount.name}</p>
                      {!discount.is_active && <Badge tone="gray">Nonaktif</Badge>}
                      {discount.requires_approval && <Badge tone="yellow">Perlu persetujuan</Badge>}
                    </div>
                    <p className="mt-0.5 text-xs text-muted">
                      {discount.discount_type === 'PERCENT'
                        ? `${Number(discount.value).toLocaleString('id-ID', { maximumFractionDigits: 2 })}%`
                        : formatMoney(discount.value)}
                      {' · '}
                      {DISCOUNT_SCOPES.find((item) => item.value === discount.scope)?.label ??
                        discount.scope}
                      {Number(discount.min_spend) > 0 && ` · min ${formatMoney(discount.min_spend)}`}
                      {discount.max_discount && ` · maks ${formatMoney(discount.max_discount)}`}
                    </p>
                  </div>
                  {canManageDiscounts && (
                    <div className="flex items-center gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setDiscountOpen(discount)}>
                        Ubah
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={toggleDiscount.isPending}
                        onClick={() =>
                          toggleDiscount.mutate({ id: discount.id, active: !discount.is_active })
                        }
                      >
                        {discount.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                      </Button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      {taxOpen && (
        <CreateTaxDialog
          loading={createTax.isPending}
          onOpenChange={(open) => !open && setTaxOpen(false)}
          onSubmit={(body) => createTax.mutate(body)}
        />
      )}

      {discountOpen && (
        <DiscountDialog
          discount={discountOpen}
          loading={saveDiscount.isPending}
          onOpenChange={(open) => !open && setDiscountOpen(null)}
          onSubmit={(id, body) => saveDiscount.mutate({ id, body })}
        />
      )}
    </>
  )
}

function emptyDiscount(): Discount {
  return {
    id: '',
    code: '',
    name: '',
    discount_type: 'PERCENT',
    value: '',
    scope: 'ORDER',
    min_spend: '0',
    requires_approval: false,
    is_active: true,
  }
}

/**
 * Aturan outlet: inklusif/eksklusif, service kena pajak, pembulatan, dan
 * ambang persetujuan. Disimpan sebagai satu PATCH agar tidak ada keadaan
 * setengah tersimpan.
 *
 * Form dikelola sebagai draft lokal di komponen anak yang di-remount lewat
 * `key` saat data server berubah — pola "reset state with a key" — supaya
 * tidak perlu menyinkronkan state di dalam effect.
 */
function OutletRulesCard(props: {
  settings?: OutletSettings
  loading: boolean
  canEdit: boolean
  onSave: (body: Partial<OutletSettings>) => Promise<unknown>
  onSaved: () => void
  onSuccess: () => void
  onError: (error: unknown) => void
}) {
  const { settings, loading } = props
  if (loading || !settings) {
    return (
      <Card className="lg:col-span-5">
        <CardHeader title="Aturan transaksi" />
        <div className="space-y-3 p-5">
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-10 w-full" />
          ))}
        </div>
      </Card>
    )
  }

  return <OutletRulesForm key={settingsKey(settings)} {...props} initial={settings} />
}

function settingsKey(settings: OutletSettings): string {
  return [
    settings.tax_inclusive,
    settings.tax_on_service,
    settings.rounding_mode,
    settings.discount_cashier_max_percent,
    settings.discount_approval_threshold_amount,
    settings.refund_approval_threshold,
  ].join('|')
}

function OutletRulesForm({
  initial,
  canEdit,
  onSave,
  onSaved,
  onSuccess,
  onError,
}: {
  initial: OutletSettings
  canEdit: boolean
  onSave: (body: Partial<OutletSettings>) => Promise<unknown>
  onSaved: () => void
  onSuccess: () => void
  onError: (error: unknown) => void
}) {
  const [draft, setDraft] = useState<OutletSettings>(initial)
  const [saving, setSaving] = useState(false)

  async function handleSave() {
    setSaving(true)
    try {
      await onSave({
        tax_inclusive: draft.tax_inclusive,
        tax_on_service: draft.tax_on_service,
        rounding_mode: draft.rounding_mode,
        discount_cashier_max_percent: draft.discount_cashier_max_percent,
        discount_approval_threshold_amount: draft.discount_approval_threshold_amount,
        refund_approval_threshold: draft.refund_approval_threshold,
      })
      onSaved()
      onSuccess()
    } catch (error) {
      onError(error)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card className="lg:col-span-5">
      <CardHeader title="Aturan transaksi" />
      <div className="space-y-4 p-5">
        <label className="flex items-start gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="mt-0.5 size-4 accent-brand-600"
            checked={draft.tax_inclusive}
            disabled={!canEdit}
            onChange={(event) => setDraft({ ...draft, tax_inclusive: event.target.checked })}
          />
          <span>
            Harga menu sudah termasuk pajak
            <span className="block text-xs text-muted">
              Bila mati, pajak ditambahkan di atas subtotal.
            </span>
          </span>
        </label>

        <label className="flex items-start gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="mt-0.5 size-4 accent-brand-600"
            checked={draft.tax_on_service}
            disabled={!canEdit}
            onChange={(event) => setDraft({ ...draft, tax_on_service: event.target.checked })}
          />
          <span>
            Pajak dihitung setelah service charge
            <span className="block text-xs text-muted">
              Bila mati, pajak hanya dihitung dari subtotal.
            </span>
          </span>
        </label>

        <Select
          label="Pembulatan total"
          value={draft.rounding_mode}
          disabled={!canEdit}
          onValueChange={(value) => setDraft({ ...draft, rounding_mode: value })}
          options={ROUNDING_MODES.map((item) => ({ value: item.value, label: item.label }))}
        />

        <Input
          label="Batas diskon kasir (%)"
          inputMode="decimal"
          value={trimNumber(draft.discount_cashier_max_percent)}
          disabled={!canEdit}
          onChange={(event) =>
            setDraft({ ...draft, discount_cashier_max_percent: event.target.value })
          }
          helper="Diskon di atas angka ini butuh persetujuan."
        />

        <NumberInput
          label="Ambang persetujuan diskon"
          value={toDigits(draft.discount_approval_threshold_amount)}
          disabled={!canEdit}
          onValueChange={(value) =>
            setDraft({ ...draft, discount_approval_threshold_amount: value || '0' })
          }
        />

        <NumberInput
          label="Ambang persetujuan refund"
          value={toDigits(draft.refund_approval_threshold)}
          disabled={!canEdit}
          onValueChange={(value) => setDraft({ ...draft, refund_approval_threshold: value || '0' })}
        />

        {canEdit && (
          <Button onClick={handleSave} loading={saving} className="w-full">
            Simpan aturan
          </Button>
        )}
      </div>
    </Card>
  )
}

/** "75000.00" -> "75000" agar NumberInput menampilkan digit utuh. */
function toDigits(value: string): string {
  return value.split('.')[0] ?? '0'
}

/** "15.0000" -> "15"; angka bulat dibersihkan dari nol di belakang koma. */
function trimNumber(value: string): string {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return value
  return String(parsed)
}

/** Tanggal berlaku pajak (tanpa jam): "16 Nov 2026". */
function formatISODate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    timeZone: 'Asia/Jakarta',
  }).format(date)
}

function CreateTaxDialog({
  loading,
  onOpenChange,
  onSubmit,
}: {
  loading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (body: {
    type: string
    name: string
    rate_percent: string
    applies_to: string[]
    effective_from: string | null
  }) => void
}) {
  const [type, setType] = useState('TAX')
  const [name, setName] = useState('')
  const [rate, setRate] = useState('')
  const [appliesTo, setAppliesTo] = useState<string[]>([])
  const [effectiveFrom, setEffectiveFrom] = useState('')

  const rateValid = /^\d+(\.\d{1,4})?$/.test(rate.trim()) && Number(rate) <= 100

  return (
    <Dialog
      open
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title="Tambah tarif"
      description="Tarif baru langsung aktif setelah disimpan."
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button
            loading={loading}
            disabled={!rateValid || name.trim() === ''}
            onClick={() =>
              onSubmit({
                type,
                name: name.trim(),
                rate_percent: rate.trim(),
                applies_to: appliesTo,
                effective_from: effectiveFrom || null,
              })
            }
          >
            Simpan
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Select
          label="Jenis"
          value={type}
          onValueChange={setType}
          options={TAX_TYPES.map((item) => ({ value: item.value, label: item.label }))}
        />
        <Input
          label="Nama"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="PPN"
        />
        <Input
          label="Tarif (%)"
          inputMode="decimal"
          value={rate}
          onChange={(event) => setRate(event.target.value)}
          error={rate !== '' && !rateValid ? 'Masukkan angka 0–100.' : undefined}
        />
        <Input
          label="Berlaku sejak"
          type="date"
          value={effectiveFrom}
          onChange={(event) => setEffectiveFrom(event.target.value)}
          helper="Kosongkan untuk mulai hari ini."
        />
        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">Berlaku untuk</legend>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {SERVICE_TYPES.map((item) => (
              <label key={item.value} className="flex min-h-11 items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-brand-600"
                  checked={appliesTo.includes(item.value)}
                  onChange={(event) =>
                    setAppliesTo((prev) =>
                      event.target.checked
                        ? [...prev, item.value]
                        : prev.filter((code) => code !== item.value),
                    )
                  }
                />
                {item.label}
              </label>
            ))}
          </div>
          <p className="mt-1 text-xs text-muted">
            Tidak memilih apa pun berarti berlaku untuk semua jenis layanan.
          </p>
        </fieldset>
      </div>
    </Dialog>
  )
}

function DiscountDialog({
  discount,
  loading,
  onOpenChange,
  onSubmit,
}: {
  discount: Discount
  loading: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (id: string | undefined, body: DiscountInput) => void
}) {
  const isEdit = discount.id !== ''
  const [code, setCode] = useState(discount.code)
  const [name, setName] = useState(discount.name)
  const [type, setType] = useState(discount.discount_type)
  const [value, setValue] = useState(discount.value.split('.')[0] ?? '')
  const [scope, setScope] = useState(discount.scope)
  const [minSpend, setMinSpend] = useState(toDigits(discount.min_spend))
  const [maxDiscount, setMaxDiscount] = useState(
    discount.max_discount ? toDigits(discount.max_discount) : '',
  )
  const [requiresApproval, setRequiresApproval] = useState(discount.requires_approval)

  const valueValid = /^\d+(\.\d{1,4})?$/.test(value.trim()) && Number(value) > 0
  const canSubmit = code.trim() !== '' && name.trim() !== '' && valueValid

  return (
    <Dialog
      open
      onOpenChange={(next) => !loading && onOpenChange(next)}
      title={isEdit ? 'Ubah promo' : 'Tambah promo'}
      description="Diskon persen maksimal 100. Kosongkan batas maksimal bila tidak perlu."
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={loading}>
            Batal
          </Button>
          <Button
            loading={loading}
            disabled={!canSubmit}
            onClick={() =>
              onSubmit(isEdit ? discount.id : undefined, {
                code: code.trim().toUpperCase(),
                name: name.trim(),
                discount_type: type,
                value: value.trim(),
                scope,
                min_spend: minSpend || '0',
                max_discount: maxDiscount || null,
                starts_at: null,
                ends_at: null,
                days_of_week: null,
                requires_approval: requiresApproval,
              })
            }
          >
            Simpan
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Input
          label="Kode"
          value={code}
          onChange={(event) => setCode(event.target.value.toUpperCase())}
          placeholder="PROMO10"
        />
        <Input
          label="Nama"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Promo 10%"
        />
        <Select
          label="Jenis"
          value={type}
          onValueChange={setType}
          options={DISCOUNT_TYPES.map((item) => ({ value: item.value, label: item.label }))}
        />
        <Input
          label={type === 'PERCENT' ? 'Nilai (%)' : 'Nilai (Rp)'}
          inputMode="decimal"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          error={value !== '' && !valueValid ? 'Masukkan angka lebih dari 0.' : undefined}
        />
        <Select
          label="Cakupan"
          value={scope}
          onValueChange={setScope}
          options={DISCOUNT_SCOPES.map((item) => ({ value: item.value, label: item.label }))}
        />
        <NumberInput
          label="Minimal belanja"
          value={minSpend}
          onValueChange={setMinSpend}
          helper="0 berarti tanpa minimal."
        />
        <NumberInput
          label="Maksimal potongan"
          value={maxDiscount}
          onValueChange={setMaxDiscount}
          helper="Kosongkan bila tidak dibatasi."
        />
        <label className="flex items-center gap-2 text-sm text-ink">
          <input
            type="checkbox"
            className="size-4 accent-brand-600"
            checked={requiresApproval}
            onChange={(event) => setRequiresApproval(event.target.checked)}
          />
          Perlu persetujuan sebelum dipakai
        </label>
      </div>
    </Dialog>
  )
}
