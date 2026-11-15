import { PageHeader } from '../../components/shared/PageHeader'
import { Card } from '../../components/ui/Card'

export interface PlaceholderPageProps {
  title: string
  permission: string
  description?: string
}

/**
 * Kerangka halaman fitur. Setiap halaman diganti implementasinya pada fase
 * fitur; rute sudah terdaftar sekarang agar navigasi dan guard bisa diuji.
 */
export function PlaceholderPage({ title, permission, description }: PlaceholderPageProps) {
  return (
    <>
      <PageHeader
        title={title}
        description={description ?? 'Halaman ini diisi pada fase fitur berikutnya.'}
      />
      <Card className="mt-6">
        <div className="m-5 rounded-card border border-dashed border-line px-5 py-10">
          <p className="text-sm text-muted">Belum ada data untuk ditampilkan.</p>
          <p className="mt-1 text-xs text-muted">
            Izin yang dibutuhkan: <span className="num">{permission}</span>
          </p>
        </div>
      </Card>
    </>
  )
}
