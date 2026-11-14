import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// cn menggabungkan class Tailwind; kelas terakhir menang saat konflik.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}