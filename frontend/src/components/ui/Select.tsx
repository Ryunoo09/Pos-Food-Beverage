import * as RadixSelect from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'

export interface SelectOption {
  value: string
  label: string
}

export interface SelectProps {
  label?: string
  placeholder?: string
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  disabled?: boolean
}

export function Select({
  label,
  placeholder = 'Pilih',
  value,
  onValueChange,
  options,
  disabled,
}: SelectProps) {
  return (
    <div className="w-full">
      {label && <p className="mb-1 text-sm font-medium text-ink ">{label}</p>}
      <RadixSelect.Root value={value} onValueChange={onValueChange} disabled={disabled}>
        <RadixSelect.Trigger
          className="flex h-11 w-full items-center justify-between rounded-control border border-control bg-surface px-3 text-sm text-ink outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-600/20 disabled:opacity-50   "
          aria-label={label}
        >
          <RadixSelect.Value placeholder={placeholder} />
          <RadixSelect.Icon>
            <ChevronDown className="size-4 text-muted" />
          </RadixSelect.Icon>
        </RadixSelect.Trigger>
        <RadixSelect.Portal>
          <RadixSelect.Content
            position="popper"
            sideOffset={4}
            className="z-50 max-h-64 min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-control border border-line bg-surface shadow-lg  "
          >
            <RadixSelect.Viewport className="p-1">
              {options.map((option) => (
                <RadixSelect.Item
                  key={option.value}
                  value={option.value}
                  className="flex min-h-11 cursor-pointer items-center justify-between rounded-chip px-3 text-sm text-ink outline-none data-[highlighted]:bg-raised  "
                >
                  <RadixSelect.ItemText>{option.label}</RadixSelect.ItemText>
                  <RadixSelect.ItemIndicator>
                    <Check className="size-4 text-brand-600" />
                  </RadixSelect.ItemIndicator>
                </RadixSelect.Item>
              ))}
            </RadixSelect.Viewport>
          </RadixSelect.Content>
        </RadixSelect.Portal>
      </RadixSelect.Root>
    </div>
  )
}
