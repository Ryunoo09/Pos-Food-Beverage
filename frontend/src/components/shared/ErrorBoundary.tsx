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
      <main className="grid min-h-dvh place-items-center p-6 text-center">
        <div>
          <h1 className="text-xl font-semibold">Terjadi kesalahan pada tampilan</h1>
          <p className="mt-1 max-w-md text-slate-600 dark:text-slate-300">{this.state.error.message}</p>
          <button
            type="button"
            className="mt-6 min-h-11 rounded-lg bg-brand-600 px-4 font-semibold text-white"
            onClick={() => window.location.reload()}
          >
            Muat ulang halaman
          </button>
        </div>
      </main>
    )
  }
}
