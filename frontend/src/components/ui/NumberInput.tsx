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
  const describedBy = error ? `${id}-error` : helper ? `${id}-helper` : undefined

  return (
    <div className="w-full">
      {label && (
        <label htmlFor={id} className="mb-1 block text-sm font-medium text-ink ">
          {label}
        </label>
      )}
      <div className="relative">
        <span className="absolute inset-y-0 left-0 grid w-11 place-items-center text-sm text-muted">
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
          aria-describedby={describedBy}
          onChange={(event) => onValueChange(event.target.value.replace(/\D/g, '').slice(0, 14))}
          className={cn(
            'h-11 w-full rounded-control border bg-surface pr-3 pl-11 text-right text-ink tabular-nums outline-none focus:ring-2  ',
            error
              ? 'border-negative focus:border-negative focus:ring-negative/20'
              : 'border-control focus:border-brand-600 focus:ring-brand-600/20 ',
          )}
        />
      </div>
      {error ? (
        <p id={`${id}-error`} className="mt-1 text-sm text-negative">
          {error}
        </p>
      ) : helper ? (
        <p id={`${id}-helper`} className="mt-1 text-sm text-muted">
          {helper}
        </p>
      ) : null}
    </div>
  )
}

function groupDigits(value: string): string {
  if (!value) return ''
  return value.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}
