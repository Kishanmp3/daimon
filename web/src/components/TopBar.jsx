import styles from './TopBar.module.css'

export default function TopBar({ title }) {
  const now = new Date()
  const dateStr = now.toLocaleDateString('en-US', {
    weekday: 'short',
    month:   'short',
    day:     'numeric',
    year:    'numeric',
  })

  return (
    <div className={styles.bar}>
      <div className={styles.spacer} />
      <h1 className={styles.title}>{title}</h1>
      <div className={styles.right}>
        <span className={styles.date}>{dateStr}</span>
      </div>
    </div>
  )
}
