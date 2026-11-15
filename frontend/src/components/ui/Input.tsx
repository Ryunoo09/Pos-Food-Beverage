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
        <label htmlFor={inputId} className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200">
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
            'h-11 w-full rounded-lg border bg-white px-3 text-slate-900 outline-none',
            'placeholder:text-slate-400 focus:ring-2',
            'dark:bg-slate-900 dark:text-slate-100',
            error
              ? 'border-red-500 focus:border-red-500 focus:ring-red-500/30'
              : 'border-slate-300 focus:border-brand-500 focus:ring-brand-500/30 dark:border-slate-700',
            trailing && 'pr-11',
            className,
          )}
        />
        {trailing && <div className="absolute inset-y-0 right-0 grid w-11 place-items-center">{trailing}</div>}
      </div>
      {error ? (
        <p id={`${inputId}-error`} className="mt-1 text-sm text-red-600">
          {error}
        </p>
      ) : helper ? (
        <p id={`${inputId}-helper`} className="mt-1 text-sm text-slate-500">
          {helper}
        </p>
      ) : null}
    </div>
  )
}
