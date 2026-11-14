import { expect, test } from 'vitest'
import { formatMoney } from './formatMoney'

// Intl memakai non-breaking space (U+00A0) antara "Rp" dan angka.
const RP = 'Rp\u00A0'

test('formatMoney merender nominal bulat dengan pemisah ribuan', () => {
  expect(formatMoney('47300.00')).toBe(`${RP}47.300`)
})

test('formatMoney merender nol', () => {
  expect(formatMoney('0')).toBe(`${RP}0`)
})

test('formatMoney mempertahankan desimal di belakang koma', () => {
  expect(formatMoney('1234.56')).toBe(`${RP}1.234,56`)
})

test('formatMoney merender nominal juta', () => {
  expect(formatMoney('1500000')).toBe(`${RP}1.500.000`)
})
