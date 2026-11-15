import type { InputHTMLAttributes, ReactNode } from 'react'
import { useId } from 'react'
import { cn } from '../../utils/cn'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helper?: string
  trailing?: ReactNode
}

export function Input({ label, error, helper, trailing, className, id, ...props }: InputProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const describedBy = error ? `${inputId}-error` : helper ? `${inputId}-helper` : undefined

  return (
    <div className="w-full">
      {label && (
        <label htmlFor={inputId} className="mb-2 block text-sm font-medium text-ink">
          {label}
        </label>
      )}
      <div className="relative">
        <input
          {...props}
          id={inputId}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn(
            'h-11 w-full rounded-control border bg-surface px-3 text-ink outline-none',
            'placeholder:text-faint focus:ring-2',
            error
              ? 'border-negative focus:border-negative focus:ring-negative/20'
              : 'border-control focus:border-brand-600 focus:ring-brand-600/20',
            trailing && 'pr-11',
            className,
          )}
        />
        {trailing && <div className="absolute inset-y-0 right-0 grid w-11 place-items-center">{trailing}</div>}
      </div>
      {error ? (
        <p id={`${inputId}-error`} className="mt-1.5 text-sm text-negative">
          {error}
        </p>
      ) : helper ? (
        <p id={`${inputId}-helper`} className="mt-1.5 text-sm text-muted">
          {helper}
        </p>
      ) : null}
    </div>
  )
}
