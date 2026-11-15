import { zodResolver } from '@hookform/resolvers/zod'
import { Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useLocation, useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { authApi } from '../../api/auth'
import { Button } from '../../components/ui/Button'
import { Input } from '../../components/ui/Input'
import { useAuthStore } from '../../store/auth'
import type { ApiError } from '../../types/api.types'

const schema = z.object({
  username: z.string().min(1, 'Username wajib diisi.'),
  password: z.string().min(1, 'Password wajib diisi.'),
})

type FormValues = z.infer<typeof schema>

export function LoginPage() {
  const [showPassword, setShowPassword] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const setAuth = useAuthStore((state) => state.setAuth)
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: { username: '', password: '' } })

  async function onSubmit(values: FormValues) {
    setFormError(null)
    try {
      const { data } = await authApi.login(values)
      setAuth(data.data.user, data.data.access_token, data.data.permissions)
      const target = data.data.user.must_change_password
        ? '/change-password'
        : (from ?? '/dashboard')
      navigate(target, { replace: true })
    } catch (error) {
      setFormError(loginErrorMessage(error as ApiError))
    }
  }

  return (
    <main className="grid min-h-dvh place-items-center bg-slate-100 p-4 dark:bg-slate-950">
      <form
        className="w-full max-w-sm rounded-xl bg-white p-6 shadow dark:bg-slate-900"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
      >
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Masuk</h1>
        <p className="mt-1 text-sm text-slate-500">Food POS UMKM</p>

        <div className="mt-6 space-y-4">
          <Input
            label="Username"
            autoComplete="username"
            autoFocus
            error={errors.username?.message}
            {...register('username')}
          />
          <Input
            label="Password"
            type={showPassword ? 'text' : 'password'}
            autoComplete="current-password"
            error={errors.password?.message}
            trailing={
              <button
                type="button"
                onClick={() => setShowPassword((value) => !value)}
                aria-label={showPassword ? 'Sembunyikan password' : 'Tampilkan password'}
                className="grid size-11 place-items-center text-slate-500"
              >
                {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            }
            {...register('password')}
          />
        </div>

        {formError && (
          <p role="alert" className="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {formError}
          </p>
        )}

        <Button type="submit" size="lg" className="mt-6 w-full" loading={isSubmitting}>
          Masuk
        </Button>
      </form>
    </main>
  )
}

function loginErrorMessage(error: ApiError): string {
  if (error.code === 'AUTH_ACCOUNT_LOCKED') {
    const minutes = error.retryAfter ? Math.ceil(error.retryAfter / 60) : 15
    return `Akun terkunci sementara. Coba lagi dalam ${minutes} menit.`
  }
  if (error.code === 'AUTH_INVALID_CREDENTIALS') {
    return 'Username atau password salah.'
  }
  if (error.code === 'AUTH_ACCOUNT_INACTIVE') {
    return 'Akun tidak aktif. Hubungi pemilik usaha.'
  }
  return error.message
}
