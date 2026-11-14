import { useState } from 'react'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  return (
    <main className="flex min-h-dvh items-center justify-center bg-slate-100 p-4">
      <form
        className="w-full max-w-sm rounded-xl bg-white p-6 shadow"
        onSubmit={(event) => {
          event.preventDefault()
          alert('Login belum terhubung ke API (fase setup).')
        }}
      >
        <h1 className="text-2xl font-semibold text-slate-900">Masuk</h1>

        <label className="mt-6 block text-sm font-medium text-slate-700" htmlFor="username">
          Username
        </label>
        <input
          id="username"
          name="username"
          autoComplete="username"
          className="mt-1 h-12 w-full rounded-lg border border-slate-300 px-3 text-slate-900 outline-none focus:border-orange-500 focus:ring-2 focus:ring-orange-500/30"
          value={username}
          onChange={(event) => setUsername(event.target.value)}
        />

        <label className="mt-4 block text-sm font-medium text-slate-700" htmlFor="password">
          Password
        </label>
        <input
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          className="mt-1 h-12 w-full rounded-lg border border-slate-300 px-3 text-slate-900 outline-none focus:border-orange-500 focus:ring-2 focus:ring-orange-500/30"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />

        <button
          type="submit"
          className="mt-6 h-12 w-full rounded-lg bg-orange-600 font-semibold text-white hover:bg-orange-700"
        >
          Masuk
        </button>
      </form>
    </main>
  )
}
