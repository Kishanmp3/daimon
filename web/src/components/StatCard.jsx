import styles from './StatCard.module.css'

export default function StatCard({ label, value, highlight, delay = 0 }) {
  return (
    <div
      className={`${styles.card} ${highlight ? styles.highlight : ''}`}
      style={{ animationDelay: `${delay}ms` }}
    >
      <span className={styles.label}>{label}</span>
      <span className={styles.value}>{value}</span>
    </div>
  )
}
