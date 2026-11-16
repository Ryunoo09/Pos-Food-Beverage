import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { inventoryApi } from '../../api/inventory'
import { recipeApi } from '../../api/recipes'
import type { Recipe, RecipeItemInput } from '../../api/recipes'
import { PageHeader } from '../../components/shared/PageHeader'
import { Badge } from '../../components/ui/Badge'
import { Button } from '../../components/ui/Button'
import { Card, CardBody, CardHeader } from '../../components/ui/Card'
import { Input } from '../../components/ui/Input'
import { Select } from '../../components/ui/Select'
import { Spinner } from '../../components/ui/Spinner'
import { useToast } from '../../hooks/useToast'
import { useAuthStore } from '../../store/auth'
import { apiMessage } from '../../utils/apiError'
import { formatMoney } from '../../utils/formatMoney'

interface DraftLine {
  ingredient_id: string
  quantity: string
  waste_factor_percent: string
}

export function RecipePage() {
  const { variantId = '' } = useParams()
  const toast = useToast()
  const queryClient = useQueryClient()
  const permissions = useAuthStore((state) => state.permissions)
  const canCreate = permissions.includes('recipes.create')
  const canSeeCost = permissions.includes('recipes.view_cost')

  // Editor di-hidrasi dari server tanpa effect: `draft` bernilai null berarti "belum
  // disentuh" dan tampilan memakai data resep aktif. Setelah pengguna mengubah atau
  // menyimpan, `draft` memuat nilai lokal.
  const [draft, setDraft] = useState<{ lines: DraftLine[]; yieldNote: string } | null>(null)

  // 404 RECIPE_NOT_FOUND berarti varian belum punya resep aktif; itu kondisi
  // normal, bukan error yang perlu ditampilkan.
  const active = useQuery({
    queryKey: ['recipe', variantId],
    queryFn: async () => {
      try {
        return (await recipeApi.active(variantId)).data.data
      } catch (error) {
        const apiError = error as Partial<{ code: string }> | null
        if (apiError?.code === 'RECIPE_NOT_FOUND') return null
        throw error
      }
    },
    enabled: variantId !== '',
  })

  const versions = useQuery({
    queryKey: ['recipe-versions', variantId],
    queryFn: async () => (await recipeApi.versions(variantId)).data.data,
    enabled: variantId !== '',
  })

  const ingredients = useQuery({
    queryKey: ['ingredients', { active: true, q: '' }],
    queryFn: async () => (await inventoryApi.ingredients.list({ is_active: true, per_page: 100 })).data.data,
  })

  const existingLines: DraftLine[] = (active.data?.items ?? []).map((item) => ({
    ingredient_id: item.ingredient_id,
    quantity: item.quantity,
    waste_factor_percent: item.waste_factor_percent,
  }))
  const lines = draft?.lines ?? existingLines
  const yieldNote = draft?.yieldNote ?? (active.data?.yield_note ?? '')

  function updateLines(update: (current: DraftLine[]) => DraftLine[]) {
    setDraft({ lines: update(lines), yieldNote })
  }

  const save = useMutation({
    mutationFn: () => {
      const items: RecipeItemInput[] = lines
        .filter((line) => line.ingredient_id !== '' && Number(line.quantity) > 0)
        .map((line) => ({
          ingredient_id: line.ingredient_id,
          quantity: line.quantity.trim(),
          waste_factor_percent: line.waste_factor_percent.trim() || '0',
        }))
      return recipeApi.create(variantId, { yield_note: yieldNote.trim() || null, items })
    },
    onSuccess: async () => {
      // Setelah tersimpan, editor kembali mengikuti resep aktif yang baru.
      setDraft(null)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['recipe', variantId] }),
        queryClient.invalidateQueries({ queryKey: ['recipe-versions', variantId] }),
      ])
      toast.push('Versi resep baru disimpan.')
    },
    onError: (error) => toast.push(apiMessage(error), 'error'),
  })

  const ingredientOptions = (ingredients.data ?? []).map((item) => ({
    value: item.id,
    label: `${item.name} · ${item.base_unit_code}`,
  }))

  const duplicate = new Set<string>()
  const seen = new Set<string>()
  for (const line of lines) {
    if (line.ingredient_id === '') continue
    if (seen.has(line.ingredient_id)) duplicate.add(line.ingredient_id)
    seen.add(line.ingredient_id)
  }

  const validLines = lines.filter(
    (line) => line.ingredient_id !== '' && Number(line.quantity) > 0 && !duplicate.has(line.ingredient_id),
  )
  const canSave = validLines.length > 0 && validLines.length === lines.filter((line) => line.ingredient_id !== '').length

  return (
    <>
      <Link
        to="/menu"
        className="inline-flex min-h-11 items-center gap-2 rounded-control px-2 text-sm text-muted hover:bg-raised hover:text-ink"
      >
        <ArrowLeft className="size-4" strokeWidth={1.5} />
        Menu
      </Link>

      <div className="mt-2">
        <PageHeader
          title="Resep varian"
          description="Komposisi bahan per satu porsi. Menyimpan membuat versi baru; versi lama tidak diubah."
          action={
            canCreate && (
              <Button
                loading={save.isPending}
                disabled={!canSave}
                onClick={() => save.mutate()}
              >
                Simpan versi baru
              </Button>
            )
          }
        />
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        {active.data ? (
          <>
            <Badge tone="green">Resep aktif v{active.data.version_no}</Badge>
            {canSeeCost && active.data.total_cost && (
              <span className="text-sm text-muted">
                Biaya bahan {formatMoney(active.data.total_cost)} per porsi
              </span>
            )}
          </>
        ) : (
          !active.isLoading && <Badge tone="gray">Belum ada resep aktif</Badge>
        )}
      </div>

      {active.isLoading ? (
        <div className="grid place-items-center py-16">
          <Spinner label="Memuat resep" />
        </div>
      ) : active.isError ? (
        <Card className="mt-4 p-6 text-sm text-negative">{apiMessage(active.error)}</Card>
      ) : (
        <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <Card>
            <CardHeader
              title="Komposisi bahan"
              action={
                canCreate && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() =>
                      updateLines((current) => [
                        ...current,
                        { ingredient_id: '', quantity: '', waste_factor_percent: '0' },
                      ])
                    }
                  >
                    <Plus className="size-4" strokeWidth={2} />
                    Bahan
                  </Button>
                )
              }
            />
            <CardBody className={lines.length === 0 ? 'text-sm text-muted' : 'space-y-3'}>
              {lines.length === 0 ? (
                canCreate ? (
                  <p>Belum ada bahan. Tambahkan minimal satu bahan untuk menyimpan resep.</p>
                ) : (
                  <p>Varian ini belum punya resep.</p>
                )
              ) : (
                lines.map((line, index) => {
                  const conflicted = duplicate.has(line.ingredient_id)
                  const unit =
                    (ingredients.data ?? []).find((item) => item.id === line.ingredient_id)
                      ?.base_unit_code ?? ''
                  return (
                    <div
                      key={index}
                      className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_8rem_7rem_auto] sm:items-end"
                    >
                      <Select
                        ariaLabel={`Bahan baris ${index + 1}`}
                        placeholder="Pilih bahan"
                        value={line.ingredient_id}
                        disabled={!canCreate}
                        onValueChange={(value) =>
                          updateLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, ingredient_id: value } : item,
                            ),
                          )
                        }
                        options={ingredientOptions}
                      />
                      <Input
                        aria-label={`Jumlah baris ${index + 1}`}
                        inputMode="decimal"
                        placeholder="0"
                        value={line.quantity}
                        disabled={!canCreate}
                        onChange={(event) =>
                          updateLines((current) =>
                            current.map((item, i) =>
                              i === index ? { ...item, quantity: event.target.value } : item,
                            ),
                          )
                        }
                        trailing={unit ? <span className="text-xs text-muted">{unit}</span> : undefined}
                      />
                      <Input
                        aria-label={`Waste baris ${index + 1}`}
                        inputMode="decimal"
                        placeholder="0"
                        value={line.waste_factor_percent}
                        disabled={!canCreate}
                        onChange={(event) =>
                          updateLines((current) =>
                            current.map((item, i) =>
                              i === index
                                ? { ...item, waste_factor_percent: event.target.value }
                                : item,
                            ),
                          )
                        }
                        trailing={<span className="text-xs text-muted">%</span>}
                      />
                      {canCreate && (
                        <Button
                          variant="ghost"
                          className="text-negative"
                          aria-label={`Hapus bahan baris ${index + 1}`}
                          onClick={() =>
                            updateLines((current) => current.filter((_item, i) => i !== index))
                          }
                        >
                          <Trash2 className="size-4" strokeWidth={1.5} />
                        </Button>
                      )}
                      {conflicted && (
                        <p className="text-xs text-negative sm:col-span-4">
                          Bahan ini muncul lebih dari sekali.
                        </p>
                      )}
                    </div>
                  )
                })
              )}
            </CardBody>
          </Card>

          <div className="space-y-4">
            <Card>
              <CardHeader title="Catatan hasil" />
              <CardBody>
                <Input
                  label="Yield note"
                  placeholder="Contoh: 1 porsi = 250 ml"
                  value={yieldNote}
                  disabled={!canCreate}
                  onChange={(event) => setDraft({ lines, yieldNote: event.target.value })}
                />
                <p className="mt-2 text-xs text-muted">
                  Waste factor menaikkan kebutuhan bahan: jumlah efektif = jumlah × (1 + waste%).
                </p>
              </CardBody>
            </Card>

            <Card>
              <CardHeader title="Riwayat versi" />
              <CardBody className="p-0">
                {versions.isLoading ? (
                  <div className="p-5">
                    <Spinner label="Memuat versi" />
                  </div>
                ) : (versions.data ?? []).length === 0 ? (
                  <p className="p-5 text-sm text-muted">Belum ada versi resep.</p>
                ) : (
                  <ul className="divide-y divide-rule">
                    {(versions.data ?? []).map((version) => (
                      <VersionRow key={version.id} version={version} canSeeCost={canSeeCost} />
                    ))}
                  </ul>
                )}
              </CardBody>
            </Card>
          </div>
        </div>
      )}
    </>
  )
}

function VersionRow({ version, canSeeCost }: { version: Recipe; canSeeCost: boolean }) {
  return (
    <li className="flex items-center justify-between gap-3 px-5 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">Versi {version.version_no}</p>
        <p className="text-xs text-muted">{version.items.length} bahan</p>
      </div>
      <div className="text-right">
        <Badge tone={version.status === 'ACTIVE' ? 'green' : 'gray'}>
          {version.status === 'ACTIVE' ? 'Aktif' : 'Lama'}
        </Badge>
        {canSeeCost && version.total_cost && (
          <p className="num mt-1 text-xs text-muted">{formatMoney(version.total_cost)}</p>
        )}
      </div>
    </li>
  )
}
