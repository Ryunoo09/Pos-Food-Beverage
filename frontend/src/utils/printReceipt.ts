import type { Receipt } from '../api/orders'
import { formatDate } from './formatDate'
import { formatMoney } from './formatMoney'

const escape = (value: string) =>
  value.replace(/[&<>"']/g, (char) => {
    switch (char) {
      case '&':
        return '&amp;'
      case '<':
        return '&lt;'
      case '>':
        return '&gt;'
      case '"':
        return '&quot;'
      default:
        return '&#39;'
    }
  })

/** Cetak struk ukuran 80mm lewat jendela baru; tidak ada dependensi printer. */
export function printReceipt(receipt: Receipt, changeAmount?: string) {
  const { outlet, order, payments } = receipt
  const rows = (order.items ?? [])
    .filter((item) => item.status !== 'VOID')
    .map(
      (item) => `
        <tr>
          <td>
            <strong>${escape(item.product_name)}</strong>
            ${item.variant_name ? `<br><small>${escape(item.variant_name)}</small>` : ''}
            ${item.modifiers?.length ? `<br><small>${item.modifiers.map((mod) => escape(mod.option_name)).join(', ')}</small>` : ''}
            <br><small>${item.quantity} x ${formatMoney(item.unit_price)}</small>
          </td>
          <td class="right">${formatMoney(item.line_total)}</td>
        </tr>`,
    )
    .join('')

  const paymentRows = (payments ?? [])
    .map(
      (payment) =>
        `<tr><td>${escape(payment.method_code)}</td><td class="right">${formatMoney(payment.amount)}</td></tr>`,
    )
    .join('')

  const html = `<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<title>Struk ${escape(order.order_no ?? '')}</title>
<style>
  * { box-sizing: border-box; }
  body { width: 80mm; margin: 0 auto; padding: 4mm; font: 12px/1.45 ui-monospace, monospace; color: #000; }
  h1 { font-size: 15px; text-align: center; margin: 0 0 2mm; }
  p { margin: 0; }
  .center { text-align: center; }
  .right { text-align: right; }
  table { width: 100%; border-collapse: collapse; }
  td { padding: 1mm 0; vertical-align: top; }
  hr { border: 0; border-top: 1px dashed #000; margin: 2mm 0; }
  .total { font-size: 14px; font-weight: 700; }
</style>
</head>
<body>
  <h1>${escape(outlet.name)}</h1>
  ${outlet.address ? `<p class="center">${escape(outlet.address)}</p>` : ''}
  ${outlet.phone ? `<p class="center">${escape(outlet.phone)}</p>` : ''}
  <hr>
  <p>No: ${escape(order.order_no ?? '-')}</p>
  <p>Waktu: ${escape(formatDate(order.created_at))}</p>
  <p>Layanan: ${escape(order.service_type)}</p>
  <hr>
  <table>${rows}</table>
  <hr>
  <table>
    <tr><td>Subtotal</td><td class="right">${formatMoney(order.subtotal_amount)}</td></tr>
    <tr><td>Diskon</td><td class="right">-${formatMoney(order.item_discount_amount)}</td></tr>
    <tr><td>Service</td><td class="right">${formatMoney(order.service_charge_amount)}</td></tr>
    <tr><td>Pajak</td><td class="right">${formatMoney(order.tax_amount)}</td></tr>
    <tr><td>Pembulatan</td><td class="right">${formatMoney(order.rounding_amount)}</td></tr>
    <tr class="total"><td>TOTAL</td><td class="right">${formatMoney(order.total_amount)}</td></tr>
  </table>
  <hr>
  <table>${paymentRows}</table>
  ${changeAmount ? `<p>Kembalian: ${formatMoney(changeAmount)}</p>` : ''}
  <hr>
  <p class="center">Terima kasih</p>
</body>
</html>`

  const win = window.open('', '_blank', 'width=380,height=640')
  if (!win) return
  win.document.write(html)
  win.document.close()
  win.focus()
  win.print()
}
