import { expect, test } from 'vitest'
import { canAccess } from './permissions'

const perms = ['orders.view', 'products.view']

test('canAccess tanpa syarat selalu lolos', () => {
  expect(canAccess([], undefined)).toBe(true)
})

test('canAccess lolos bila permission dimiliki', () => {
  expect(canAccess(perms, 'orders.view')).toBe(true)
})

test('canAccess menolak permission yang tidak dimiliki', () => {
  expect(canAccess(perms, 'users.view')).toBe(false)
})
