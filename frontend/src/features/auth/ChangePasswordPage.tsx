import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { authApi } from '../../api/auth'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { Input } from '../../components/ui/Input'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import type { ApiError } from '../../types/api.types'

const schema = z
  .object({
    current_password: z.string().min(1, 'Password saat ini wajib diisi.'),
    new_password: z.string().min(10, 'Password baru minimal 10 karakter.'),
    confirm_password: z.string().min(1, 'Konfirmasi password wajib diisi.'),
  })
  .refine((values) => values.new_password === values.confirm_password, {
    message: 'Konfirmasi password tidak sama.',
    path: ['confirm_password'],
  })

type FormValues = z.infer<typeof schema>

export function ChangePasswordPage() {
  const clearAuth = useAuthStore((state) => state.clearAuth)
  const toast = useToast()
  const navigate = useNavigate()

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: '', new_password: '', confirm_password: '' },
  })

  async function onSubmit(values: FormValues) {
    try {
      await authApi.changePassword({
        current_password: values.current_password,
        new_password: values.new_password,
      })
      // Backend mencabut semua sesi setelah ganti password.
      clearAuth()
      toast.push('Password diubah. Silakan masuk dengan password baru.', 'success')
      navigate('/login', { replace: true })
    } catch (error) {
      const apiError = error as ApiError
      const field = apiError.code === 'VALIDATION_ERROR' ? 'new_password' : 'current_password'
      setError(field, { message: apiError.message })
    }
  }

  return (
    <main className="grid min-h-dvh place-items-center bg-slate-100 p-4 dark:bg-slate-950">
      <Card className="w-full max-w-md">
        <CardHeader title="Ganti password" />
        <CardBody>
          <p className="mb-4 text-sm text-slate-600 dark:text-slate-300">
            Password sementara wajib diganti sebelum memakai aplikasi.
          </p>
          <form className="space-y-4" onSubmit={handleSubmit(onSubmit)} noValidate>
            <Input
              label="Password saat ini"
              type="password"
              autoComplete="current-password"
              error={errors.current_password?.message}
              {...register('current_password')}
            />
            <Input
              label="Password baru"
              type="password"
              autoComplete="new-password"
              helper="Minimal 10 karakter, tidak sama dengan username."
              error={errors.new_password?.message}
              {...register('new_password')}
            />
            <Input
              label="Ulangi password baru"
              type="password"
              autoComplete="new-password"
              error={errors.confirm_password?.message}
              {...register('confirm_password')}
            />
            <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>
              Simpan password
            </Button>
          </form>
        </CardBody>
      </Card>
    </main>
  )
}
