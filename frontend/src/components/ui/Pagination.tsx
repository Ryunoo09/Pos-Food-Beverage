import { ChevronLeft, ChevronRight } from 'lucide-react'

export interface PaginationProps {
  page: number
  totalPages: number
  total: number
  onPageChange: (page: number) => void
}

export function Pagination({ page, totalPages, total, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null

  return (
    <nav className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm" aria-label="Paginasi">
      <span className="text-muted">
        Halaman {page} dari {totalPages} · {total} data
      </span>
      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          className="inline-flex min-h-11 items-center gap-1 rounded-control border border-control px-3 font-medium disabled:opacity-40 "
        >
          <ChevronLeft className="size-4" /> Sebelumnya
        </button>
        <button
          type="button"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          className="inline-flex min-h-11 items-center gap-1 rounded-control border border-control px-3 font-medium disabled:opacity-40 "
        >
          Berikutnya <ChevronRight className="size-4" />
        </button>
      </div>
    </nav>
  )
}
