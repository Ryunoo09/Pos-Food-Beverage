import { Component } from 'react'
import type { ErrorInfo, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children

    return (
      <main className="grid min-h-dvh place-items-center bg-ground p-6">
        <div className="max-w-md">
          <h1 className="text-[22px] leading-tight font-semibold tracking-[-0.01em]">
            Terjadi kesalahan pada tampilan
          </h1>
          <p className="mt-1.5 text-sm text-muted">{this.state.error.message}</p>
          <button
            type="button"
            className="mt-5 min-h-11 rounded-control bg-ink px-4 text-sm font-medium text-surface"
            onClick={() => window.location.reload()}
          >
            Muat ulang halaman
          </button>
        </div>
      </main>
    )
  }
}
