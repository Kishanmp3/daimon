import { useEffect, useState, useMemo } from 'react'
import { createPortal } from 'react-dom'
import { LineChart, Line, PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts'
import { X, Wrench, Sparkles, Zap, Database, Code } from 'lucide-react'
import TopBar  from '../components/TopBar.jsx'
import Heatmap from '../components/Heatmap.jsx'
import {
  fmtDateTime, fmtDateFull, fmtDateOnly, fmtDuration, generateHeadline,
  parseBullets, bulletIcon, langFromFiles, classifyWorkType, LANG_COLORS,
} from '../utils.js'
import styles from './History.module.css'

// ── Helpers ────────────────────────────────────────────────────────────

const EXT_COLORS = {
  js: '#ca8a04', jsx: '#ca8a04', ts: '#ca8a04', tsx: '#ca8a04',
  py: '#3b82f6', go: '#06b6d4', css: '#ec4899', scss: '#ec4899',
}

function extColor(filename) {
  const ext = (filename.split('.').pop() || '').toLowerCase()
  return EXT_COLORS[ext] || '#52525b'
}

function basename(path) {
  return path.split(/[\\/]/).pop() || path
}

const BULLET_ICONS = {
  wrench:   { Icon: Wrench,   color: '#ef4444' },
  zap:      { Icon: Zap,      color: '#f59e0b' },
  database: { Icon: Database, color: '#22c55e' },
  sparkles: { Icon: Sparkles, color: '#3b82f6' },
}

// ── Modal ──────────────────────────────────────────────────────────────

function SessionModal({ session, onClose }) {
  const files    = session.files_changed || []
  const added    = session.lines_added   || 0
  const removed  = session.lines_removed || 0
  const total    = added + removed
  const addPct   = total > 0 ? (added   / total) * 100 : 50
  const remPct   = total > 0 ? (removed / total) * 100 : 50
  const bullets  = parseBullets(session.summary)
  const langData = langFromFiles(files)
  const workTypes = classifyWorkType(session.summary)
  const sparkData = Array.from({ length: 12 }, (_, i) => ({ i, v: 0.5 }))

  const MAX_CHIPS = 8
  const visibleFiles = files.slice(0, MAX_CHIPS)
  const overflow = files.length - MAX_CHIPS

  // Close on Escape
  useEffect(() => {
    const h = e => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [onClose])

  // Lock body scroll
  useEffect(() => {
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = '' }
  }, [])

  return createPortal(
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={e => e.stopPropagation()}>

        {/* Close button */}
        <button className={styles.closeBtn} onClick={onClose}>
          <X size={16} />
        </button>

        {/* ── Modal header ── */}
        <div className={styles.modalHeader}>
          <div className={styles.modalHeaderLeft}>
            <span className={styles.modalProject}>{session.project_name}</span>
            <span className={styles.modalDate}>{fmtDateFull(session.started_at)}</span>
          </div>
          <div className={styles.modalBadges}>
            <span className={styles.badge}>{fmtDuration(session.duration_sec)}</span>
            <span className={`${styles.badge} ${session.status === 'active' ? styles.badgeActive : styles.badgeClosed}`}>
              {session.status}
            </span>
          </div>
        </div>

        <div className={styles.modalDivider} />

        {/* ── Main body ── */}
        <div className={styles.modalBody}>

          {/* Left column */}
          <div className={styles.modalLeft}>

            {/* Diff bar */}
            {total > 0 && (
              <div className={styles.section}>
                <h3 className={styles.sectionTitle}>Changes</h3>
                <div className={styles.diffBar}>
                  <div className={styles.diffAdded}   style={{ width: `${addPct}%` }} />
                  <div className={styles.diffRemoved} style={{ width: `${remPct}%` }} />
                </div>
                <div className={styles.diffLabels}>
                  <span className={styles.diffAdd}>+{added} lines added</span>
                  <span className={styles.diffRem}>−{removed} lines removed</span>
                </div>
              </div>
            )}

            {/* File chips */}
            {files.length > 0 && (
              <div className={styles.section}>
                <h3 className={styles.sectionTitle}>Files Changed</h3>
                <div className={styles.chips}>
                  {visibleFiles.map(f => (
                    <span
                      key={f}
                      className={styles.chip}
                      style={{ '--chip-color': extColor(f) }}
                    >
                      {basename(f)}
                    </span>
                  ))}
                  {overflow > 0 && (
                    <span className={`${styles.chip} ${styles.chipMore}`}>+{overflow} more</span>
                  )}
                </div>
              </div>
            )}

            {/* Activity sparkline */}
            <div className={styles.section}>
              <h3 className={styles.sectionTitle}>Session Activity</h3>
              <div className={styles.sparkWrap}>
                <LineChart width={280} height={60} data={sparkData}>
                  <Line
                    type="monotone"
                    dataKey="v"
                    stroke="var(--accent)"
                    strokeWidth={1.5}
                    dot={false}
                    isAnimationActive={false}
                    strokeOpacity={0.5}
                  />
                </LineChart>
                <p className={styles.sparkNote}>Save frequency over session — no timeline data available</p>
              </div>
            </div>

            {/* Work type breakdown */}
            {workTypes.length > 0 && (
              <div className={styles.section}>
                <h3 className={styles.sectionTitle}>Work Type</h3>
                <div className={styles.workTypes}>
                  {workTypes.map(wt => (
                    <div key={wt.label} className={styles.workRow}>
                      <span className={styles.workLabel}>{wt.label}</span>
                      <div className={styles.workTrack}>
                        <div
                          className={styles.workFill}
                          style={{ width: `${wt.pct}%`, background: wt.color }}
                        />
                      </div>
                      <span className={styles.workPct}>{wt.pct}%</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Right column */}
          <div className={styles.modalRight}>

            {/* Language pie chart */}
            {langData.length > 0 && (
              <div className={styles.section}>
                <h3 className={styles.sectionTitle}>Languages</h3>
                <div className={styles.langChartWrap}>
                  <PieChart width={160} height={160}>
                    <Pie
                      data={langData}
                      cx={75}
                      cy={75}
                      innerRadius={45}
                      outerRadius={72}
                      dataKey="value"
                      paddingAngle={2}
                    >
                      {langData.map((entry, i) => (
                        <Cell key={i} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip
                      content={({ active, payload }) => {
                        if (!active || !payload?.length) return null
                        const d = payload[0].payload
                        return (
                          <div className={styles.langTip}>
                            <span style={{ color: d.color }}>{d.name}</span>
                            <span>{d.pct}%</span>
                          </div>
                        )
                      }}
                    />
                  </PieChart>
                  <div className={styles.langLegend}>
                    {langData.map(l => (
                      <div key={l.name} className={styles.langRow}>
                        <span className={styles.langDot} style={{ background: l.color }} />
                        <span className={styles.langName}>{l.name}</span>
                        <span className={styles.langPct}>{l.pct}%</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* ── Timeline ── */}
        {bullets.length > 0 && (
          <div className={styles.timelineSection}>
            <h3 className={styles.sectionTitle}>Session Timeline</h3>
            <div className={styles.timeline}>
              {bullets.map((bullet, i) => {
                const type = bulletIcon(bullet)
                const { Icon, color } = BULLET_ICONS[type]
                return (
                  <div key={i} className={styles.timelineItem}>
                    <div className={styles.timelineLeft}>
                      <div className={styles.timelineIcon} style={{ background: `${color}18`, borderColor: `${color}40` }}>
                        <Icon size={12} color={color} strokeWidth={2} />
                      </div>
                      {i < bullets.length - 1 && <div className={styles.timelineLine} />}
                    </div>
                    <p className={styles.timelineText}>{bullet}</p>
                  </div>
                )
              })}
            </div>
          </div>
        )}

      </div>
    </div>,
    document.body
  )
}

// ── Collapsed session card ─────────────────────────────────────────────

function SessionCard({ session, delay }) {
  const [open, setOpen] = useState(false)

  const headline = generateHeadline(session.summary)
  const added    = session.lines_added   || 0
  const removed  = session.lines_removed || 0
  const total    = added + removed
  const addPct   = total > 0 ? (added   / total) * 100 : 50
  const remPct   = total > 0 ? (removed / total) * 100 : 50

  return (
    <>
      <div
        className={styles.sessionCard}
        style={{ '--delay': `${delay}ms` }}
        onClick={() => setOpen(true)}
        role="button"
        tabIndex={0}
        onKeyDown={e => e.key === 'Enter' && setOpen(true)}
      >
        {/* Row 1: project + headline + badges */}
        <div className={styles.cardTop}>
          <div className={styles.cardTopLeft}>
            <span className={styles.projName}>{session.project_name}</span>
            <span className={styles.headline}>{headline}</span>
          </div>
          <div className={styles.cardBadges}>
            <span className={styles.badge}>{fmtDuration(session.duration_sec)}</span>
            <span className={`${styles.badge} ${session.status === 'active' ? styles.badgeActive : styles.badgeClosed}`}>
              {session.status}
            </span>
          </div>
        </div>

        {/* Row 2: date */}
        <div className={styles.cardDate}>{fmtDateTime(session.started_at)}</div>

        {/* Row 3: diff bar */}
        {total > 0 && (
          <div className={styles.cardDiff}>
            <div className={styles.diffBar}>
              <div className={styles.diffAdded}   style={{ width: `${addPct}%` }} />
              <div className={styles.diffRemoved} style={{ width: `${remPct}%` }} />
            </div>
            <div className={styles.diffLabels}>
              <span className={styles.diffAdd}>+{added}</span>
              <span className={styles.diffRem}>−{removed}</span>
            </div>
          </div>
        )}
      </div>

      {open && <SessionModal session={session} onClose={() => setOpen(false)} />}
    </>
  )
}

// ── Main ───────────────────────────────────────────────────────────────

const RANGES = ['This Week', 'This Month', 'All Time']

export default function History() {
  const [sessions,  setSessions]  = useState([])
  const [heatmap,   setHeatmap]   = useState([])
  const [projects,  setProjects]  = useState([])
  const [project,   setProject]   = useState('all')
  const [range,     setRange]     = useState('All Time')
  const [search,    setSearch]    = useState('')
  const [activeDay, setActiveDay] = useState(null)

  useEffect(() => {
    fetch('/api/sessions').then(r => r.json()).then(setSessions).catch(() => {})
    fetch('/api/heatmap').then(r => r.json()).then(setHeatmap).catch(() => {})
    fetch('/api/projects').then(r => r.json()).then(setProjects).catch(() => {})
  }, [])

  const filtered = useMemo(() => {
    const now = new Date()
    return sessions.filter(s => {
      if (project !== 'all' && s.project_name !== project) return false

      if (range !== 'All Time') {
        const d = new Date(s.started_at.includes('T') ? s.started_at : s.started_at.replace(' ', 'T') + 'Z')
        const cutoff = new Date(now)
        if (range === 'This Week')  cutoff.setDate(cutoff.getDate() - 7)
        if (range === 'This Month') cutoff.setDate(cutoff.getDate() - 30)
        if (d < cutoff) return false
      }

      if (activeDay && fmtDateOnly(s.started_at) !== activeDay) return false

      if (search) {
        const q = search.toLowerCase()
        if (!s.project_name?.toLowerCase().includes(q) && !s.summary?.toLowerCase().includes(q)) return false
      }

      return true
    })
  }, [sessions, project, range, search, activeDay])

  const projectNames = [...new Set(sessions.map(s => s.project_name).filter(Boolean))]

  const heatmap3mo = useMemo(() => {
    const cutoff = new Date()
    cutoff.setDate(cutoff.getDate() - 91)
    return heatmap.filter(e => new Date(e.date) >= cutoff)
  }, [heatmap])

  return (
    <div className={styles.page}>
      <TopBar title="History" />
      <div className={styles.content}>

        {/* Filter bar */}
        <div className={styles.filterBar}>
          <select
            className={styles.select}
            value={project}
            onChange={e => setProject(e.target.value)}
          >
            <option value="all">All Projects</option>
            {projectNames.map(n => <option key={n} value={n}>{n}</option>)}
          </select>

          <div className={styles.rangeGroup}>
            {RANGES.map(r => (
              <button
                key={r}
                className={`${styles.rangeBtn} ${range === r ? styles.rangeBtnActive : ''}`}
                onClick={() => setRange(r)}
              >
                {r}
              </button>
            ))}
          </div>

          <input
            className={styles.search}
            type="text"
            placeholder="Search summaries..."
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>

        {/* Mini heatmap */}
        <div className={styles.miniHeatWrap}>
          <div className={styles.miniHeatLabel}>
            {activeDay ? (
              <>Showing <strong>{activeDay}</strong> —{' '}
                <button className={styles.clearDay} onClick={() => setActiveDay(null)}>clear</button>
              </>
            ) : 'Click a day to filter'}
          </div>
          <div className={styles.miniHeat}>
            <Heatmap data={heatmap3mo} days={91} />
          </div>
        </div>

        {/* Cards */}
        {filtered.length === 0 ? (
          <p className={styles.empty}>
            No sessions match these filters. Start a session with{' '}
            <code>breaklog daemon</code> to see your history.
          </p>
        ) : (
          <div className={styles.cards}>
            {filtered.map((s, i) => (
              <SessionCard key={s.id} session={s} delay={i * 35} />
            ))}
          </div>
        )}

      </div>
    </div>
  )
}
