import { useEffect, useState } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell,
} from 'recharts'
import TopBar from '../components/TopBar.jsx'
import { fmtHours, aggregateLangs, aggregateWorkTypes, LANG_COLORS } from '../utils.js'
import styles from './Analytics.module.css'

// ── Helpers ────────────────────────────────────────────────────────────

function fmtHour(h) {
  if (h === 0)  return '12a'
  if (h < 12)   return `${h}a`
  if (h === 12) return '12p'
  return `${h - 12}p`
}

function padDays(data) {
  const map = {}
  for (const { date, hours } of data) map[date] = hours
  return Array.from({ length: 30 }, (_, i) => {
    const d = new Date()
    d.setDate(d.getDate() - (29 - i))
    const key = d.toISOString().slice(0, 10)
    return {
      date:  key,
      label: (d.getDate() === 1 || i === 0 || i === 29)
        ? d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
        : '',
      hours: +(map[key] || 0).toFixed(2),
    }
  })
}

function padHours(data) {
  const map = {}
  for (const { hour, count } of data) map[hour] = count
  return Array.from({ length: 24 }, (_, h) => ({
    hour:  h,
    label: h % 3 === 0 ? fmtHour(h) : '',
    count: map[h] || 0,
  }))
}

// ── Tooltips ───────────────────────────────────────────────────────────

function BarTip({ active, payload }) {
  if (!active || !payload?.length) return null
  const d = payload[0]
  return (
    <div className={styles.tooltip}>
      <span className={styles.ttLabel}>{d.payload?.label || d.payload?.date}</span>
      <span className={styles.ttVal}>
        {d.dataKey === 'hours' ? fmtHours(d.value) : d.value + ' sessions'}
      </span>
    </div>
  )
}

function PieTip({ active, payload }) {
  if (!active || !payload?.length) return null
  const d = payload[0]
  return (
    <div className={styles.tooltip}>
      <span className={styles.ttLabel}>{d.name}</span>
      <span className={styles.ttVal}>
        {d.payload?.pct != null ? `${d.payload.pct}%` : fmtHours(d.value)}
      </span>
    </div>
  )
}

const DONUT_COLORS = ['#f59e0b','#b45309','#78350f','#3f3f46','#52525b','#71717a']

// ── Component ──────────────────────────────────────────────────────────

export default function Analytics() {
  const [insights,   setInsights]   = useState(null)
  const [sessions,   setSessions]   = useState([])
  const [narrative,  setNarrative]  = useState(null)
  const [loadingNar, setLoadingNar] = useState(false)

  useEffect(() => {
    fetch('/api/insights').then(r => r.json()).then(setInsights).catch(() => {})
    fetch('/api/sessions').then(r => r.json()).then(setSessions).catch(() => {})
    loadNarrative()
  }, [])

  function loadNarrative() {
    setLoadingNar(true)
    fetch('/api/insights/weekly-narrative')
      .then(r => r.json())
      .then(d => { setNarrative(d.narrative || null); setLoadingNar(false) })
      .catch(() => setLoadingNar(false))
  }

  const dayData    = padDays(insights?.hours_per_day   || [])
  const hourData   = padHours(insights?.sessions_by_hour || [])
  const projData   = (insights?.hours_by_project || []).filter(p => p.hours > 0)
  const langData   = aggregateLangs(sessions)
  const workData   = aggregateWorkTypes(sessions)

  const streak        = insights?.streak_days    || 0
  const longestStreak = insights?.longest_streak || 0
  const totalDays     = insights?.total_days_coded || 0

  return (
    <div className={styles.page}>
      <TopBar title="Analytics" />
      <div className={styles.content}>

        {/* ── Row 1 ── */}
        <div className={styles.grid}>

          {/* Hours per day */}
          <div className={styles.card} style={{ '--delay': '0ms' }}>
            <h2 className={styles.cardTitle}>Hours per Day</h2>
            <p className={styles.cardSub}>Last 30 days</p>
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={dayData} barCategoryGap="20%">
                <XAxis dataKey="label" axisLine={false} tickLine={false}
                  tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} interval={0} />
                <YAxis hide />
                <Tooltip content={<BarTip />} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                <Bar dataKey="hours" fill="var(--accent)" radius={[2, 2, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Hours by project (donut) */}
          <div className={styles.card} style={{ '--delay': '50ms' }}>
            <h2 className={styles.cardTitle}>Hours by Project</h2>
            <p className={styles.cardSub}>All time</p>
            {projData.length === 0 ? (
              <p className={styles.empty}>No data yet.</p>
            ) : (
              <div className={styles.donutWrap}>
                <ResponsiveContainer width="100%" height={200}>
                  <PieChart>
                    <Pie data={projData} cx="50%" cy="50%"
                      innerRadius={55} outerRadius={85}
                      dataKey="hours" nameKey="name" paddingAngle={2}>
                      {projData.map((_, i) => (
                        <Cell key={i} fill={DONUT_COLORS[i % DONUT_COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip content={<PieTip />} />
                  </PieChart>
                </ResponsiveContainer>
                <div className={styles.legend}>
                  {projData.slice(0, 5).map((p, i) => (
                    <div key={p.name} className={styles.legendRow}>
                      <span className={styles.legendDot} style={{ background: DONUT_COLORS[i % DONUT_COLORS.length] }} />
                      <span className={styles.legendName}>{p.name}</span>
                      <span className={styles.legendVal}>{fmtHours(p.hours)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Sessions by time of day */}
          <div className={styles.card} style={{ '--delay': '100ms' }}>
            <h2 className={styles.cardTitle}>When You Code</h2>
            <p className={styles.cardSub}>Sessions by hour of day</p>
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={hourData} barCategoryGap="15%">
                <XAxis dataKey="label" axisLine={false} tickLine={false}
                  tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} interval={0} />
                <YAxis hide />
                <Tooltip content={<BarTip />} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                <Bar dataKey="count" fill="var(--accent)" radius={[2, 2, 0, 0]} opacity={0.85} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Streak info */}
          <div className={styles.card} style={{ '--delay': '150ms' }}>
            <h2 className={styles.cardTitle}>Streaks</h2>
            <p className={styles.cardSub}>Consecutive coding days</p>
            <div className={styles.streakList}>
              <div className={styles.streakItem}>
                <span className={styles.streakNum} style={{ color: 'var(--accent)' }}>{streak}</span>
                <div className={styles.streakMeta}>
                  <span className={styles.streakLabel}>Current Streak</span>
                  <span className={styles.streakUnit}>days</span>
                </div>
              </div>
              <div className={styles.divider} />
              <div className={styles.streakItem}>
                <span className={styles.streakNum}>{longestStreak}</span>
                <div className={styles.streakMeta}>
                  <span className={styles.streakLabel}>Longest Streak</span>
                  <span className={styles.streakUnit}>days</span>
                </div>
              </div>
              <div className={styles.divider} />
              <div className={styles.streakItem}>
                <span className={styles.streakNum}>{totalDays}</span>
                <div className={styles.streakMeta}>
                  <span className={styles.streakLabel}>Total Days Coded</span>
                  <span className={styles.streakUnit}>days</span>
                </div>
              </div>
            </div>
          </div>

          {/* Languages distribution */}
          <div className={styles.card} style={{ '--delay': '200ms' }}>
            <h2 className={styles.cardTitle}>Languages</h2>
            <p className={styles.cardSub}>Across all sessions</p>
            {langData.length === 0 ? (
              <p className={styles.empty}>No file data yet.</p>
            ) : (
              <div className={styles.donutWrap}>
                <ResponsiveContainer width="100%" height={200}>
                  <PieChart>
                    <Pie data={langData} cx="50%" cy="50%"
                      innerRadius={55} outerRadius={85}
                      dataKey="value" nameKey="name" paddingAngle={2}>
                      {langData.map((entry, i) => (
                        <Cell key={i} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip content={<PieTip />} />
                  </PieChart>
                </ResponsiveContainer>
                <div className={styles.legend}>
                  {langData.slice(0, 6).map(l => (
                    <div key={l.name} className={styles.legendRow}>
                      <span className={styles.legendDot} style={{ background: l.color }} />
                      <span className={styles.legendName}>{l.name}</span>
                      <span className={styles.legendVal}>{l.pct}%</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Work type distribution */}
          <div className={styles.card} style={{ '--delay': '250ms' }}>
            <h2 className={styles.cardTitle}>Work Type</h2>
            <p className={styles.cardSub}>Across all sessions</p>
            {workData.length === 0 ? (
              <p className={styles.empty}>No session summaries yet.</p>
            ) : (
              <div className={styles.workTypes}>
                {workData.map(wt => (
                  <div key={wt.label} className={styles.workRow}>
                    <span className={styles.workLabel}>{wt.label}</span>
                    <div className={styles.workTrack}>
                      <div className={styles.workFill} style={{ width: `${wt.pct}%`, background: wt.color }} />
                    </div>
                    <span className={styles.workPct}>{wt.pct}%</span>
                  </div>
                ))}
              </div>
            )}
          </div>

        </div>

        {/* ── Weekly narrative ── */}
        <div className={styles.narrativeCard} style={{ '--delay': '300ms' }}>
          <div className={styles.narrativeHeader}>
            <div>
              <h2 className={styles.cardTitle}>This Week in Review</h2>
              <p className={styles.cardSub}>AI-generated summary of the last 7 days</p>
            </div>
            <button
              className={styles.regenBtn}
              onClick={loadNarrative}
              disabled={loadingNar}
            >
              {loadingNar ? 'Generating…' : 'Regenerate'}
            </button>
          </div>
          <div className={styles.narrativeBody}>
            {loadingNar ? (
              <span className={styles.loading}>Thinking…</span>
            ) : narrative ? (
              <p className={styles.narrativeText}>{narrative}</p>
            ) : (
              <p className={styles.empty}>No narrative available yet.</p>
            )}
          </div>
        </div>

      </div>
    </div>
  )
}
