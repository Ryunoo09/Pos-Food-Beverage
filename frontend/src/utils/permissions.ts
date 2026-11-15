/** Permission helper murni agar bisa diuji tanpa render React. */
export function canAccess(permissions: string[], required?: string): boolean {
  return required === undefined || permissions.includes(required)
}
