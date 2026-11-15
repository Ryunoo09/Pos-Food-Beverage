import { Badge } from '../../components/ui/Badge'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'

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
    <Card>
      <CardHeader title={title} action={<Badge tone="gray">{permission}</Badge>} />
      <CardBody>
        <p className="text-slate-600 dark:text-slate-300">
          {description ?? 'Halaman ini akan diisi pada fase fitur berikutnya.'}
        </p>
      </CardBody>
    </Card>
  )
}
