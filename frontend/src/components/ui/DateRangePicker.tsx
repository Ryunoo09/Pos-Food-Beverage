export interface DateRange {
  from: string
  to: string
}

export interface DateRangePickerProps {
  value: DateRange
  onChange: (value: DateRange) => void
  label?: string
}

/** Memakai <input type="date"> native agar tidak perlu library kalender. */
export function DateRangePicker({ value, onChange, label = 'Rentang tanggal' }: DateRangePickerProps) {
  const inputClass =
    'h-11 rounded-control border border-control bg-surface px-3 text-sm text-ink outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/20   '

  return (
    <fieldset className="w-full">
      <legend className="mb-1 text-sm font-medium text-ink ">{label}</legend>
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="date"
          aria-label="Dari tanggal"
          className={inputClass}
          value={value.from}
          max={value.to || undefined}
          onChange={(event) => onChange({ ...value, from: event.target.value })}
        />
        <span className="text-faint">s/d</span>
        <input
          type="date"
          aria-label="Sampai tanggal"
          className={inputClass}
          value={value.to}
          min={value.from || undefined}
          onChange={(event) => onChange({ ...value, to: event.target.value })}
        />
      </div>
    </fieldset>
  )
}
