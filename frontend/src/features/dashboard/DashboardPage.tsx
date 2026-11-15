import { Card, CardBody, CardHeader } from '../../components/ui/Card'

export function DashboardPage() {
  return (
    <Card>
      <CardHeader title="Dashboard" />
      <CardBody>
        <p className="text-slate-600 dark:text-slate-300">
          Ringkasan penjualan, pesanan aktif, dan stok kritis akan tampil di sini pada fase fitur.
        </p>
      </CardBody>
    </Card>
  )
}
