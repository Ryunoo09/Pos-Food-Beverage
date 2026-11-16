import { cn } from '../../utils/cn'

/** Bar abu-abu datar seukuran nilai akhirnya; tanpa shimmer, tanpa geser layout. */
export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('rounded-chip bg-raised', className)} />
}
