import { Link } from 'react-router-dom'

export default function Landing() {
  return (
    <div className="page">
      <div className="hero">
        <h1>Live polls that feel instant.</h1>
        <p>
          Create a poll, share a link, and watch the results update live as the
          votes roll in. No refresh. No reload. Just a pulse.
        </p>
        <div className="hero-buttons">
          <Link to="/signup" className="btn btn-primary">
            Create your first poll
          </Link>
          <Link to="/login" className="btn">
            Log in
          </Link>
        </div>
      </div>

      <div className="page-wide" style={{ display: 'grid', gap: 16, marginTop: 30 }}>
        <div className="card">
          <h2 className="card-title">How it works</h2>
          <p className="card-sub" style={{ marginBottom: 0 }}>
            1. Create a poll — a unique link is minted for you. 2. Share the link
            anywhere. 3. Everyone votes and the results stream in live through
            Redis pub/sub, rendered with zero page refreshes.
          </p>
        </div>
        <div className="card">
          <h2 className="card-title">Built to be real</h2>
          <p className="card-sub" style={{ marginBottom: 0 }}>
            Go + Gin on the backend, MongoDB for durable records, and Redis doing
            the heavy lifting for live counts and real-time fan-out.
          </p>
        </div>
      </div>
    </div>
  )
}