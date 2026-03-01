// ── Date parsing ──────────────────────────────────────────────────────

function parseDate(str) {
  if (!str) return null
  let s = str
  if (!s.includes('T')) {
    // "2006-01-02 15:04:05" → "2006-01-02T15:04:05Z"
    s = s.replace(' ', 'T') + 'Z'
  } else if (!s.endsWith('Z') && !s.includes('+')) {
    s = s + 'Z'
  }
  const d = new Date(s)
  return isNaN(d.getTime()) ? null : d
}

export function fmtDateTime(str) {
  const d = parseDate(str)
  if (!d) return 'Unknown date'
  return d.toLocaleString('en-US', {
    month: 'short', day: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

export function fmtDateFull(str) {
  const d = parseDate(str)
  if (!d) return 'Unknown date'
  return d.toLocaleDateString('en-US', {
    weekday: 'short', month: 'short', day: 'numeric',
    year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

export function fmtDateOnly(str) {
  const d = parseDate(str)
  if (!d) return 'unknown'
  return d.toISOString().slice(0, 10)
}

export function timeAgo(str) {
  const d = parseDate(str)
  if (!d) return ''
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60)    return 'just now'
  if (diff < 3600)  return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

// ── Duration / hours ───────────────────────────────────────────────────

export function fmtDuration(sec) {
  if (!sec) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h === 0) return `${m}m`
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

export function fmtHours(h) {
  if (!h || h === 0) return '0h'
  const hh = Math.floor(h)
  const mm  = Math.round((h - hh) * 60)
  if (hh === 0) return `${mm}m`
  return mm > 0 ? `${hh}h ${mm}m` : `${hh}h`
}

export function fmtElapsed(sec) {
  if (!sec || sec < 60) return `${sec || 0}s`
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h === 0) return `${m}m`
  return `${h}h ${m}m`
}

// ── Greeting ───────────────────────────────────────────────────────────

export function getGreeting() {
  const h = new Date().getHours()
  if (h <  5) return 'Good night'
  if (h < 12) return 'Good morning'
  if (h < 17) return 'Good afternoon'
  if (h < 21) return 'Good evening'
  return 'Good night'
}

// ── 4-5 word headline from summary ────────────────────────────────────

const STOP = new Set([
  'the','a','an','and','or','but','in','on','at','to','for','of','with',
  'is','was','were','are','by','that','this','it','its','be','been','have',
  'had','has','from','as','into','during','through','also','both','just',
  'added','updated','changed','modified',
])

export function generateHeadline(summary) {
  if (!summary || !summary.trim()) return 'No summary yet'
  const first = summary.split(/(?<=[.!?])\s/)[0].trim()
  const words  = first.split(/\s+/).filter(w => w.length > 1)
  const meaningful = words.filter(w => !STOP.has(w.toLowerCase()))
  const chosen = (meaningful.length >= 3 ? meaningful : words).slice(0, 5)
  return chosen.join(' ') || 'Session summary'
}

// ── Parse summary into sentences / bullets ────────────────────────────

export function parseBullets(summary) {
  if (!summary) return []
  return summary
    .split(/(?<=[.!?])\s+/)
    .map(s => s.trim())
    .filter(s => s.length > 8)
    .slice(0, 5)
}

// ── Bullet icon type ───────────────────────────────────────────────────

export function bulletIcon(text) {
  const t = text.toLowerCase()
  if (/fix|bug|debug|error|issue|crash|patch|resolv/.test(t)) return 'wrench'
  if (/refactor|clean|simplif|restructur|optim|improv|migrat/.test(t)) return 'zap'
  if (/database|schema|query|sql|table|migrat/.test(t)) return 'database'
  return 'sparkles'
}

// ── Language distribution from files_changed ──────────────────────────

const LANG_MAP = {
  js:   'JavaScript', jsx:  'JavaScript',
  ts:   'TypeScript', tsx:  'TypeScript',
  py:   'Python',
  go:   'Go',
  rs:   'Rust',
  java: 'Java',
  kt:   'Kotlin',
  css:  'CSS', scss: 'CSS', sass: 'CSS',
  html: 'HTML', htm: 'HTML',
  json: 'JSON',
  yaml: 'YAML', yml: 'YAML',
  md:   'Markdown',
  sql:  'SQL',
  sh:   'Shell', bash: 'Shell',
  c:    'C/C++', cpp: 'C/C++', h: 'C/C++',
}

export const LANG_COLORS = {
  JavaScript: '#ca8a04', TypeScript: '#3b82f6', Python:  '#3b82f6',
  Go:         '#06b6d4', CSS:        '#ec4899', HTML:    '#f97316',
  JSON:       '#8b5cf6', SQL:        '#22c55e', Shell:   '#71717a',
  Rust:       '#f97316', Kotlin:     '#7c3aed', Java:    '#dc2626',
  Markdown:   '#6b7280', YAML:       '#6b7280', 'C/C++': '#64748b',
  Other:      '#52525b',
}

export function langFromFiles(files) {
  const counts = {}
  for (const f of (files || [])) {
    const parts = f.split('.')
    const ext   = parts.length > 1 ? parts.pop().toLowerCase() : ''
    const lang  = LANG_MAP[ext] || 'Other'
    counts[lang] = (counts[lang] || 0) + 1
  }
  const total = Object.values(counts).reduce((s, v) => s + v, 0)
  if (!total) return []
  return Object.entries(counts)
    .map(([name, count]) => ({
      name,
      value: count,
      pct:   Math.round(count / total * 100),
      color: LANG_COLORS[name] || LANG_COLORS.Other,
    }))
    .sort((a, b) => b.value - a.value)
}

// Aggregate lang distribution across many sessions
export function aggregateLangs(sessions) {
  const allFiles = sessions.flatMap(s => s.files_changed || [])
  return langFromFiles(allFiles)
}

// ── Work type classification ───────────────────────────────────────────

const WORK_DEFS = [
  { label: 'Frontend', color: '#f59e0b', kw: ['component','ui','style','css','layout','design','button','form','page','view','modal','sidebar','nav','react','jsx','html','animation','responsive','frontend','render','display','theme'] },
  { label: 'Backend',  color: '#3b82f6', kw: ['api','endpoint','route','server','handler','middleware','auth','logic','backend','request','response','http','rest','service','controller'] },
  { label: 'Database', color: '#22c55e', kw: ['sql','schema','migration','query','database','table','model','db','sqlite','index','column','record'] },
  { label: 'DevOps',   color: '#8b5cf6', kw: ['config','build','deploy','docker','ci','pipeline','env','setup','install','package','dependency','script'] },
  { label: 'Bug Fix',  color: '#ef4444', kw: ['fix','bug','debug','error','issue','crash','broken','patch','resolv','incorrect','wrong','failure','null'] },
]

export function classifyWorkType(summary) {
  if (!summary) return []
  const lower = summary.toLowerCase()
  const scored = WORK_DEFS.map(d => ({
    label: d.label, color: d.color,
    score: d.kw.filter(k => lower.includes(k)).length,
  })).filter(x => x.score > 0)

  if (!scored.length) return [{ label: 'General', color: '#71717a', pct: 100 }]
  const total = scored.reduce((s, x) => s + x.score, 0)
  return scored
    .map(x => ({ ...x, pct: Math.round(x.score / total * 100) }))
    .sort((a, b) => b.pct - a.pct)
}

// Aggregate work types across many sessions
export function aggregateWorkTypes(sessions) {
  const totals = {}
  const colors = {}
  for (const s of sessions) {
    const wts = classifyWorkType(s.summary)
    for (const wt of wts) {
      totals[wt.label] = (totals[wt.label] || 0) + wt.pct
      colors[wt.label] = wt.color
    }
  }
  const sum = Object.values(totals).reduce((a, b) => a + b, 0)
  if (!sum) return []
  return Object.entries(totals)
    .map(([label, val]) => ({ label, color: colors[label], pct: Math.round(val / sum * 100) }))
    .sort((a, b) => b.pct - a.pct)
}
