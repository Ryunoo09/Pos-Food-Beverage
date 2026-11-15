import { useId } from 'react'
import { cn } from '../../utils/cn'

export interface NumberInputProps {
  label?: string
  value: string
  onValueChange: (value: string) => void
  error?: string
  helper?: string
  disabled?: boolean
  placeholder?: string
}

/**
 * Input uang. Nilai yang dikirim ke API selalu string digit ("47300"), bukan
 * float. ponytail: hanya rupiah bulat; tambah desimal bila ada harga berkoma.
 */
export function NumberInput({
  label,
  value,
  onValueChange,
  error,
  helper,
  disabled,
  placeholder = '0',
}: NumberInputProps) {
  const id = useId()

  return (
    <div className="w-full">
      {label && (
        <label htmlFor={id} className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200">
          {label}
        </label>
      )}
      <div className="relative">
        <span className="absolute inset-y-0 left-0 grid w-11 place-items-center text-sm text-slate-500">
          Rp
        </span>
        <input
          id={id}
          inputMode="numeric"
          autoComplete="off"
          value={groupDigits(value)}
          disabled={disabled}
          placeholder={placeholder}
          aria-invalid={error ? true : undefined}
          onChange={(event) => onValueChange(event.target.value.replace(/\D/g, '').slice(0, 14))}
          className={cn(
            'h-11 w-full rounded-lg border bg-white pr-3 pl-11 text-right text-slate-900 tabular-nums outline-none focus:ring-2 dark:bg-slate-900 dark:text-slate-100',
            error
              ? 'border-red-500 focus:border-red-500 focus:ring-red-500/30'
              : 'border-slate-300 focus:border-brand-500 focus:ring-brand-500/30 dark:border-slate-700',
          )}
        />
      </div>
      {error ? (
        <p className="mt-1 text-sm text-red-600">{error}</p>
      ) : helper ? (
        <p className="mt-1 text-sm text-slate-500">{helper}</p>
      ) : null}
    </div>
  )
}

function groupDigits(value: string): string {
  if (!value) return ''
  return value.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}
