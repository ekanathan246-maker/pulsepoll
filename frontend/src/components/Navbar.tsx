import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'

export function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  return (
    <nav className="nav">
      <Link to="/" className="brand">
        <span className="pulse" />
        PulsePoll
      </Link>
      <div className="nav-links">
        {user ? (
          <>
            <span className="user-chip">{user.name}</span>
            <Link to="/dashboard" className="btn btn-ghost">
              Dashboard
            </Link>
            <button className="btn btn-ghost" onClick={handleLogout}>
              Logout
            </button>
          </>
        ) : (
          <>
            <Link to="/login" className="btn btn-ghost">
              Log in
            </Link>
            <Link to="/signup" className="btn btn-primary">
              Sign up
            </Link>
          </>
        )}
      </div>
    </nav>
  )
}