import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../utils/cn'
import { Spinner } from './Spinner'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'
type Size = 'sm' | 'md' | 'lg'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
}

const VARIANTS: Record<Variant, string> = {
  // Satu aksi utama per layar, isi near-black; brand tidak dipakai di chrome.
  primary: 'bg-ink text-surface hover:opacity-90',
  secondary: 'border border-control bg-surface text-ink hover:bg-raised',
  danger: 'bg-negative text-white hover:opacity-90',
  ghost: 'text-muted hover:bg-raised hover:text-ink',
}

const SIZES: Record<Size, string> = {
  sm: 'min-h-9 px-3 text-[13px]',
  md: 'min-h-11 px-4 text-sm',
  lg: 'min-h-13 px-6 text-base',
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  className,
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-control font-medium transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50',
        VARIANTS[variant],
        SIZES[size],
        className,
      )}
    >
      {loading && <Spinner className="size-4" />}
      {children}
    </button>
  )
}
