import type { ReactNode } from 'react'
import { cn } from '../../utils/cn'
import { Skeleton } from './Skeleton'

export interface Column<T> {
  key: string
  header: ReactNode
  render: (row: T) => ReactNode
  className?: string
}

export interface DataTableProps<T> {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  loading?: boolean
  error?: string
  onRetry?: () => void
  empty?: string
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  error,
  onRetry,
  empty = 'Belum ada data.',
}: DataTableProps<T>) {
  return (
    // min-w-max membuat tabel tetap selebar isinya sehingga kolom tidak
    // diperas; di layar sempit tabel menggeser horizontal, bukan terpotong.
    <div className="overflow-x-auto">
      <table className="w-full min-w-max text-left text-sm">
        <thead className="text-xs text-muted">
          <tr>
            {columns.map((column) => (
              <th
                key={column.key}
                scope="col"
                className={cn('px-4 py-2.5 font-normal whitespace-nowrap', column.className)}
              >
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-rule">
          {loading &&
            Array.from({ length: 5 }, (_, index) => (
              <tr key={`skeleton-${index}`}>
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3">
                    <Skeleton className="h-4 w-full" />
                  </td>
                ))}
              </tr>
            ))}

          {!loading && error && (
            <tr>
              <td colSpan={columns.length} className="px-4 py-8 text-center">
                <p className="text-sm text-negative">{error}</p>
                {onRetry && (
                  <button
                    type="button"
                    onClick={onRetry}
                    className="mt-2 min-h-11 text-sm font-medium text-ink underline underline-offset-4"
                  >
                    Coba lagi
                  </button>
                )}
              </td>
            </tr>
          )}

          {!loading && !error && rows.length === 0 && (
            <tr>
              <td colSpan={columns.length} className="px-4 py-8 text-center text-sm text-muted">
                {empty}
              </td>
            </tr>
          )}

          {!loading &&
            !error &&
            rows.map((row) => (
              <tr key={rowKey(row)} className="hover:bg-raised">
                {columns.map((column) => (
                  <td key={column.key} className={cn('px-4 py-3', column.className)}>
                    {column.render(row)}
                  </td>
                ))}
              </tr>
            ))}
        </tbody>
      </table>
    </div>
  )
}
