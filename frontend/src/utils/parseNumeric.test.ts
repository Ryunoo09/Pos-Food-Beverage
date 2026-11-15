import { expect, test } from 'vitest'
import { parseNumeric } from './parseNumeric'

test('parseNumeric menerima angka bulat', () => {
  expect(parseNumeric('47300')).toBe('47300')
})

test('parseNumeric menerima desimal NUMERIC', () => {
  expect(parseNumeric('1234.56')).toBe('1234.56')
})

test('parseNumeric memangkas spasi', () => {
  expect(parseNumeric(' 12 ')).toBe('12')
})

test('parseNumeric menolak pemisah ribuan dan teks', () => {
  expect(parseNumeric('47,300')).toBeNull()
  expect(parseNumeric('1.234.567')).toBeNull()
  expect(parseNumeric('12,5')).toBeNull()
  expect(parseNumeric('abc')).toBeNull()
  expect(parseNumeric('')).toBeNull()
})
