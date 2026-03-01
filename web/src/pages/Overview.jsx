import { useEffect, useState, useCallback } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
  LineChart, Line,
} from 'recharts'
import TopBar   from '../components/TopBar.jsx'
import StatCard from '../components/StatCard.jsx'
import Heatmap  from '../components/Heatmap.jsx'
import {
  fmtHours, fmtDuration, fmtElapsed, timeAgo, getGreeting,
} from '../utils.js'
import styles from './Overview.module.css'

// ── Chart tooltip ──────────────────────────────────────────────────────

function ChartTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null
  return (
    <div className={styles.chartTooltip}>
      <span className={styles.tooltipLabel}>{label}</span>
      <span className={styles.tooltipVal}>{fmtHours(payload[0]?.value)}</span>
    </div>
  )
}

// ── Sparkline for project row ──────────────────────────────────────────

function Sparkline({ data }) {
  const points = (data || []).map((h, i) => ({ i, h }))
  return (
    <LineChart width={60} height={24} data={points}>
      <Line
        type="monotone"
        dataKey="h"
        stroke="var(--accent)"
        strokeWidth={1.5}
        dot={false}
        isAnimationActive={false}
      />
    </LineChart>
  )
}

// ── Build Mon–Sun chart from heatmap data ──────────────────────────────

function buildWeekChart(heatmapData) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const dow    = today.getDay() === 0 ? 6 : today.getDay() - 1
  const monday = new Date(today)
  monday.setDate(monday.getDate() - dow)

  const map = {}
  for (const { date, hours } of heatmapData) map[date] = hours

  return ['Mon','Tue','Wed','Thu','Fri','Sat','Sun'].map((label, i) => {
    const d = new Date(monday)
    d.setDate(d.getDate() + i)
    const key = d.toISOString().slice(0, 10)
    return { label, hours: +(map[key] || 0).toFixed(2) }
  })
}

// ── Component ──────────────────────────────────────────────────────────

export default function Overview() {
  const [overview, setOverview] = useState(null)
  const [sessions, setSessions] = useState([])
  const [heatmap,  setHeatmap]  = useState([])
  const [projects, setProjects] = useState([])

  const fetchAll = useCallback(() => {
    fetch('/api/overview').then(r => r.json()).then(setOverview).catch(() => {})
    fetch('/api/sessions').then(r => r.json()).then(d => setSessions(d.slice(0, 5))).catch(() => {})
    fetch('/api/heatmap').then(r => r.json()).then(setHeatmap).catch(() => {})
    fetch('/api/projects').then(r => r.json()).then(setProjects).catch(() => {})
  }, [])

  useEffect(() => {
    fetchAll()
    const id = setInterval(fetchAll, 30_000)
    return () => clearInterval(id)
  }, [fetchAll])

  const mostActiveDay = (() => {
    if (!heatmap.length) return '—'
    const best = heatmap.reduce((a, b) => (a.hours > b.hours ? a : b))
    return new Date(best.date + 'T00:00:00').toLocaleDateString('en-US', { weekday: 'long' })
  })()

  const longestSec = sessions.reduce((m, s) => Math.max(m, s.duration_sec || 0), 0)
  const avgSec     = sessions.length
    ? sessions.reduce((s, x) => s + (x.duration_sec || 0), 0) / sessions.length
    : 0

  const weekChart = buildWeekChart(heatmap)
  const ov = overview || {}

  return (
    <div className={styles.page}>
      <TopBar title="Overview" />
      <div className={styles.content}>

        {/* ── Greeting header ── */}
        <div className={styles.greeting}>
          <h2 className={styles.greetingTitle}>
            {getGreeting()}, <span className={styles.greetingName}>Kishan.</span>
          </h2>
          <p className={styles.greetingSub}>Here's what you've been building.</p>
        </div>

        {/* ── 6 stat cards ── */}
        <div className={styles.statRow}>
          <StatCard label="Today's Hours"   value={fmtHours(ov.hours_today)}     highlight delay={0}   />
          <StatCard label="This Week"        value={fmtHours(ov.hours_this_week)}  delay={50}  />
          <StatCard label="Total Sessions"   value={ov.total_sessions ?? '—'}      delay={100} />
          <StatCard label="Active Projects"  value={ov.active_projects ?? '—'}     delay={150} />
          <StatCard label="Current Streak"   value={ov.streak_days ? `${ov.streak_days}d` : '—'} delay={200} />
          <StatCard label="Files Today"      value={ov.files_changed_today ?? '—'} delay={250} />
        </div>

        {/* ── Active session banner ── */}
        {ov.is_active && (
          <div className={styles.activeBanner}>
            <span className={styles.activeDot} />
            <span>
              Coding on <strong>{ov.active_project || 'unknown'}</strong>
              {' — '}
              {fmtElapsed(ov.active_elapsed_sec)}
            </span>
          </div>
        )}

        {/* ── Two-column layout ── */}
        <div className={styles.twoCol}>

          {/* LEFT 60% */}
          <div className={styles.left}>
            <section className={styles.card} style={{ '--delay': '80ms' }}>
              <h2 className={styles.cardTitle}>Recent Sessions</h2>
              {sessions.length === 0 ? (
                <p className={styles.empty}>
                  Start a session with <code>breaklog daemon</code> to see your history.
                </p>
              ) : (
                <div className={styles.sessionList}>
                  {sessions.map(s => (
                    <div key={s.id} className={styles.sessionMini}>
                      <div className={styles.sessionMiniTop}>
                        <span className={styles.sessionProject}>{s.project_name}</span>
                        <span className={styles.sessionTime}>{timeAgo(s.started_at)}</span>
                      </div>
                      <div className={styles.sessionMiniBot}>
                        <span className={styles.sessionDur}>{fmtDuration(s.duration_sec)}</span>
                        {s.summary && (
                          <span className={styles.sessionSummary}>
                            {s.summary.split('.')[0]}.
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className={styles.card} style={{ '--delay': '130ms' }}>
              <h2 className={styles.cardTitle}>This Week</h2>
              <div className={styles.chartWrap}>
                <ResponsiveContainer width="100%" height={200}>
                  <BarChart data={weekChart} barCategoryGap="30%">
                    <XAxis
                      dataKey="label"
                      axisLine={false}
                      tickLine={false}
                      tick={{ fill: 'var(--text-secondary)', fontSize: 11 }}
                    />
                    <YAxis hide />
                    <Tooltip
                      content={<ChartTooltip />}
                      cursor={{ fill: 'rgba(255,255,255,0.03)' }}
                    />
                    <Bar dataKey="hours" fill="var(--accent)" radius={[3, 3, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </section>
          </div>

          {/* RIGHT 40% */}
          <div className={styles.right}>
            <section className={styles.card} style={{ '--delay': '100ms' }}>
              <h2 className={styles.cardTitle}>Projects</h2>
              {projects.length === 0 ? (
                <p className={styles.empty}>No projects registered yet.</p>
              ) : (
                <div className={styles.projectList}>
                  {projects.map(p => (
                    <div key={p.id} className={styles.projectRow}>
                      <div className={styles.projectInfo}>
                        <span className={styles.projectName}>{p.name}</span>
                        <span className={styles.projectMeta}>
                          {p.last_active ? timeAgo(p.last_active) : 'never'}
                          {' · '}
                          {fmtDuration(p.total_sec)}
                        </span>
                      </div>
                      <Sparkline data={p.sparkline} />
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className={styles.card} style={{ '--delay': '150ms' }}>
              <h2 className={styles.cardTitle}>Quick Stats</h2>
              <div className={styles.quickList}>
                <div className={styles.quickRow}>
                  <span className={styles.quickLabel}>Most active day</span>
                  <span className={styles.quickVal}>{mostActiveDay}</span>
                </div>
                <div className={styles.quickRow}>
                  <span className={styles.quickLabel}>Longest session</span>
                  <span className={styles.quickVal}>{longestSec ? fmtDuration(longestSec) : '—'}</span>
                </div>
                <div className={styles.quickRow}>
                  <span className={styles.quickLabel}>Avg session</span>
                  <span className={styles.quickVal}>{avgSec ? fmtDuration(Math.round(avgSec)) : '—'}</span>
                </div>
              </div>
            </section>
          </div>
        </div>

        {/* ── Activity heatmap — hugs content, left-aligned ── */}
        <section className={styles.card} style={{ '--delay': '200ms' }}>
          <h2 className={styles.cardTitle}>Activity</h2>
          {heatmap.length === 0 ? (
            <p className={styles.empty}>No activity recorded yet.</p>
          ) : (
            <div className={styles.heatmapWrap}>
              <Heatmap data={heatmap} days={365} />
            </div>
          )}
        </section>

      </div>
    </div>
  )
}
