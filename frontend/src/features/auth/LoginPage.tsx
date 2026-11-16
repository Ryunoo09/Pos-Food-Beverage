import { zodResolver } from '@hookform/resolvers/zod'
import { Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useLocation, useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { authApi } from '../../api/auth'
import { ProductMark } from '../../components/shared/ProductMark'
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
    <main className="grid min-h-dvh place-items-center bg-ground p-4">
      <div className="w-full max-w-[400px] rounded-sheet border border-line bg-surface p-8">
        <div className="flex items-center gap-2.5">
          <ProductMark className="size-6 text-brand-600" />
          <span className="text-sm font-semibold">Food POS</span>
          <span className="text-[11px] text-muted">UMKM</span>
        </div>

        <h1 className="mt-8 text-[22px] leading-tight font-semibold tracking-[-0.01em]">Masuk ke kasir</h1>
        <p className="mt-1.5 text-sm text-muted">Gunakan akun yang diberikan pemilik usaha.</p>

        <form className="mt-6" onSubmit={handleSubmit(onSubmit)} noValidate>
          <div className="space-y-4">
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
                  className="grid size-11 place-items-center text-muted hover:text-ink"
                >
                  {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              }
              {...register('password')}
            />
          </div>

          {formError && (
            <p role="alert" className="mt-4 rounded-control bg-pill-negative px-3 py-2 text-sm text-pill-negative-ink">
              {formError}
            </p>
          )}

          <Button type="submit" size="lg" className="mt-6 w-full" loading={isSubmitting}>
            Masuk
          </Button>
        </form>

        <p className="mt-6 border-t border-rule pt-4 text-xs text-muted">
          Lupa password? Pemilik usaha bisa mereset dari menu Pengguna.
        </p>
      </div>
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
