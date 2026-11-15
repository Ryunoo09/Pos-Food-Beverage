import { useEffect, useRef, useState } from 'react'

export interface LineChartProps {
  points: { label: string; value: number }[]
  ticks: number[]
  formatTick: (value: number) => string
  formatValue: (value: number) => string
  /** Satu baris keterangan di titik tertinggi. */
  annotation: (point: { label: string; value: number }) => string
  seriesName: string
}

const LABEL_EVERY = 3

export function LineChart({
  points,
  ticks,
  formatTick,
  formatValue,
  annotation,
  seriesName,
}: LineChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [hover, setHover] = useState<number | null>(null)

  useEffect(() => {
    const element = containerRef.current
    if (!element) return
    const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  const compact = width > 0 && width < 520
  const padLeft = compact ? 46 : 64
  const height = compact ? 180 : 220
  const maxTick = ticks[ticks.length - 1] ?? 1
  const plotWidth = Math.max(width - padLeft - 8, 0)
  const plotHeight = height - 38 - 28
  const step = points.length > 1 ? plotWidth / (points.length - 1) : 0

  const x = (index: number) => padLeft + index * step
  const y = (value: number) => 38 + plotHeight * (1 - value / maxTick)

  const path = points.map((point, index) => `${index === 0 ? 'M' : 'L'}${x(index)},${y(point.value)}`).join(' ')

  const peakIndex = points.reduce(
    (best, point, index) => (point.value > (points[best]?.value ?? -1) ? index : best),
    0,
  )
  const peak = points[peakIndex]
  const active = hover === null ? null : points[hover]

  /** Label jangan keluar dari tepi plot. */
  const anchorAt = (position: number) => {
    if (position > width - 90) return 'end' as const
    if (position < padLeft + 40) return 'start' as const
    return 'middle' as const
  }

  return (
    <div ref={containerRef} className="relative">
      {width > 0 && (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={`${seriesName}, ${points.length} titik dari ${points[0]?.label} sampai ${points[points.length - 1]?.label}`}
          onMouseLeave={() => setHover(null)}
          onMouseMove={(event) => {
            const box = event.currentTarget.getBoundingClientRect()
            const index = Math.round((event.clientX - box.left - padLeft) / (step || 1))
            setHover(Math.min(Math.max(index, 0), points.length - 1))
          }}
        >
          <g className="text-muted">
            {ticks.map((tick) => (
              <g key={tick}>
                <line
                  x1={padLeft}
                  x2={width - 8}
                  y1={y(tick)}
                  y2={y(tick)}
                  className="stroke-line"
                  strokeWidth={1}
                />
                <text
                  x={padLeft - 10}
                  y={y(tick)}
                  textAnchor="end"
                  dominantBaseline="middle"
                  className="fill-current text-[11px] tabular-nums"
                >
                  {formatTick(tick)}
                </text>
              </g>
            ))}
            <line
              x1={padLeft}
              x2={width - 8}
              y1={y(0)}
              y2={y(0)}
              className="stroke-control"
              strokeWidth={1}
            />
            {points.map((point, index) =>
              index % LABEL_EVERY === 0 ? (
                <text
                  key={point.label}
                  x={x(index)}
                  y={height - 8}
                  textAnchor="middle"
                  className="fill-current text-[11px]"
                >
                  {point.label}
                </text>
              ) : null,
            )}
          </g>

          <path d={path} className="stroke-chart-1" strokeWidth={2} fill="none" strokeLinejoin="round" />

          {peak && (
            <g>
              <line
                x1={x(peakIndex)}
                x2={x(peakIndex)}
                y1={y(peak.value) - 6}
                y2={18}
                className="stroke-control"
                strokeWidth={1}
              />
              <text
                x={Math.min(Math.max(x(peakIndex), padLeft + 2), width - 6)}
                y={12}
                textAnchor={anchorAt(x(peakIndex))}
                className="fill-current text-[11px] font-medium text-muted"
              >
                {annotation(peak)}
              </text>
            </g>
          )}

          {active && hover !== null && (
            <g>
              <circle cx={x(hover)} cy={y(active.value)} r={4} className="fill-chart-1" />
              <text
                x={Math.min(Math.max(x(hover), padLeft + 2), width - 6)}
                y={y(active.value) - 12}
                textAnchor={anchorAt(x(hover))}
                className="fill-current text-[11px] font-medium"
              >
                {formatValue(active.value)}
              </text>
            </g>
          )}
        </svg>
      )}
    </div>
  )
}
