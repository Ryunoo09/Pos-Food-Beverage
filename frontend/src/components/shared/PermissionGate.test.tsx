// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, expect, test } from 'vitest'
import type { ReactNode } from 'react'
import { PermissionGate } from './PermissionGate'
import { useAuthStore } from '../../store/auth'

let container: HTMLDivElement
let root: Root

beforeEach(() => {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
  useAuthStore.setState({ permissions: [] })
})

async function render(ui: ReactNode) {
  await act(async () => {
    root.render(<MemoryRouter>{ui}</MemoryRouter>)
  })
  return container.textContent ?? ''
}

test('menampilkan children bila permission dimiliki', async () => {
  useAuthStore.setState({ permissions: ['orders.view'] })

  const text = await render(
    <PermissionGate perm="orders.view">
      <p>isi halaman</p>
    </PermissionGate>,
  )

  expect(text).toContain('isi halaman')
})

test('menampilkan fallback bila permission tidak dimiliki', async () => {
  useAuthStore.setState({ permissions: ['products.view'] })

  const text = await render(
    <PermissionGate perm="orders.view" fallback={<p>tidak punya akses</p>}>
      <p>isi halaman</p>
    </PermissionGate>,
  )

  expect(text).toContain('tidak punya akses')
  expect(text).not.toContain('isi halaman')
})

test('mengalihkan alih-alih merender children saat redirect diisi', async () => {
  useAuthStore.setState({ permissions: [] })

  const text = await render(
    <PermissionGate perm="users.view" redirect="/dashboard">
      <p>isi halaman</p>
    </PermissionGate>,
  )

  expect(text).not.toContain('isi halaman')
})