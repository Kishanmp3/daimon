import { Routes, Route, Navigate } from 'react-router-dom'
import Sidebar from './components/Sidebar.jsx'
import Overview from './pages/Overview.jsx'
import History from './pages/History.jsx'
import Analytics from './pages/Analytics.jsx'
import styles from './App.module.css'

export default function App() {
  return (
    <div className={styles.shell}>
      <Sidebar />
      <div className={styles.main}>
        <Routes>
          <Route path="/"          element={<Overview />}  />
          <Route path="/history"   element={<History />}   />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="*"          element={<Navigate to="/" replace />} />
        </Routes>
      </div>
    </div>
  )
}
