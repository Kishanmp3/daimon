import { NavLink } from 'react-router-dom'
import { LayoutDashboard, History, BarChart2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import styles from './Sidebar.module.css'

const NAV = [
  { to: '/',           icon: LayoutDashboard, label: 'Overview'  },
  { to: '/history',    icon: History,         label: 'History'   },
  { to: '/analytics',  icon: BarChart2,        label: 'Analytics' },
]

export default function Sidebar() {
  const [isActive, setIsActive] = useState(false)

  useEffect(() => {
    const poll = () =>
      fetch('/api/overview')
        .then(r => r.json())
        .then(d => setIsActive(!!d.is_active))
        .catch(() => {})

    poll()
    const id = setInterval(poll, 30_000)
    return () => clearInterval(id)
  }, [])

  return (
    <aside className={styles.sidebar}>
      <div className={styles.brand}>
        <span className={styles.brandName}>breaklog</span>
        <span className={styles.brandSub}>session tracker</span>
      </div>

      <nav className={styles.nav}>
        {NAV.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `${styles.navItem}${isActive ? ` ${styles.active}` : ''}`
            }
          >
            <Icon size={15} strokeWidth={1.75} />
            <span>{label}</span>
          </NavLink>
        ))}
      </nav>

      <div className={styles.statusRow}>
        <span className={`${styles.dot} ${isActive ? styles.dotGreen : styles.dotGray}`} />
        <span className={styles.statusText}>
          {isActive ? 'session active' : 'idle'}
        </span>
      </div>
    </aside>
  )
}
