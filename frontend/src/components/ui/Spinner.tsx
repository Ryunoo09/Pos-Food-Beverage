import { cn } from '../../utils/cn'

export function Spinner({ className, label }: { className?: string; label?: string }) {
  return (
    <span className="inline-flex items-center gap-2" role="status">
      <span
        className={cn(
          'size-5 animate-spin rounded-full border-2 border-control border-t-brand-600',
          className,
        )}
      />
      <span className={label ? 'text-sm text-muted' : 'sr-only'}>{label ?? 'Memuat'}</span>
    </span>
  )
}
