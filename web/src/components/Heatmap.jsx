import styles from './Heatmap.module.css'

const CELL = 10
const GAP  = 2
const STEP = CELL + GAP

function cellColor(hours) {
  if (!hours || hours === 0) return '#27272a'
  if (hours < 1)             return '#78350f'
  if (hours < 3)             return '#b45309'
  return '#f59e0b'
}

function toDateStr(d) {
  return d.toISOString().slice(0, 10)
}

export default function Heatmap({ data = [], days = 365 }) {
  // Build a map of date → hours
  const map = {}
  for (const { date, hours } of data) map[date] = hours

  // Determine start: go back `days` calendar days from today
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  // Align start to Monday of the week containing (today - days + 1)
  const rawStart = new Date(today)
  rawStart.setDate(rawStart.getDate() - (days - 1))
  const startDow = rawStart.getDay() === 0 ? 7 : rawStart.getDay() // Mon=1..Sun=7
  rawStart.setDate(rawStart.getDate() - (startDow - 1)) // rewind to Monday

  // Build cells: fill from rawStart to today
  const cells = []
  const cur = new Date(rawStart)
  while (cur <= today) {
    const key = toDateStr(cur)
    cells.push({ date: key, hours: map[key] || 0 })
    cur.setDate(cur.getDate() + 1)
  }

  // Arrange into weeks (columns of 7)
  const weeks = []
  for (let i = 0; i < cells.length; i += 7) {
    weeks.push(cells.slice(i, i + 7))
  }

  // Build month labels
  const monthLabels = []
  let lastMonth = -1
  weeks.forEach((week, wi) => {
    const firstDay = week[0]
    if (!firstDay) return
    const d = new Date(firstDay.date + 'T00:00:00')
    const m = d.getMonth()
    if (m !== lastMonth) {
      lastMonth = m
      monthLabels.push({
        col: wi,
        label: d.toLocaleDateString('en-US', { month: 'short' }),
      })
    }
  })

  const gridWidth  = weeks.length * STEP - GAP
  const gridHeight = 7 * STEP - GAP
  const labelHeight = 16

  return (
    <div className={styles.wrap}>
      <svg
        width={gridWidth}
        height={labelHeight + gridHeight}
        className={styles.svg}
      >
        {/* Month labels */}
        {monthLabels.map(({ col, label }) => (
          <text
            key={`${col}-${label}`}
            x={col * STEP}
            y={11}
            className={styles.monthLabel}
          >
            {label}
          </text>
        ))}

        {/* Cells */}
        {weeks.map((week, wi) =>
          week.map((cell, di) => {
            const color   = cellColor(cell.hours)
            const isLit   = cell.hours > 0
            const x       = wi * STEP
            const y       = labelHeight + di * STEP
            return (
              <rect
                key={cell.date}
                x={x}
                y={y}
                width={CELL}
                height={CELL}
                rx={2}
                fill={color}
                className={isLit ? styles.lit : undefined}
              >
                <title>{cell.date}: {cell.hours ? cell.hours.toFixed(1) + 'h' : 'no activity'}</title>
              </rect>
            )
          })
        )}
      </svg>
    </div>
  )
}
