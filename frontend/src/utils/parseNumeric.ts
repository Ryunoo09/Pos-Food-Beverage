/**
 * Validasi string NUMERIC dari input pengguna tanpa mengubahnya menjadi float.
 * Mengembalikan null bila format tidak valid.
 */
export function parseNumeric(value: string): string | null {
  const trimmed = value.trim()
  if (!/^-?\d+(\.\d+)?$/.test(trimmed)) return null
  return trimmed
}
