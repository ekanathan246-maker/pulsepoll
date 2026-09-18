import { Link } from 'react-router-dom'

const demoAnswers = [
	{ label: 'Ship it Friday', value: 64 },
	{ label: 'One more test', value: 27 },
	{ label: 'Ask me Monday', value: 9 },
]

export default function Landing() {
	return (
		<main className="landing-shell">
			<section className="hero page-wide">
				<div className="hero-copy">
					<p className="eyebrow"><span className="signal-dot" /> The room has a pulse</p>
					<h1>Ask the room.<br /><em>Watch it answer.</em></h1>
					<p className="hero-lede">
						Turn a question into a live moment. Share one link, collect one honest vote per browser,
						and see the result move without touching refresh.
					</p>
					<div className="hero-buttons">
						<Link to="/signup" className="btn btn-primary">Start a live poll <span aria-hidden="true">↗</span></Link>
						<Link to="/login" className="btn btn-ghost">I already host polls</Link>
					</div>
					<div className="hero-proof" aria-label="Product qualities">
						<span>No app for voters</span><span>Live recovery</span><span>Free to run</span>
					</div>
				</div>

				<div className="signal-board" aria-label="Example live poll result">
					<div className="board-topline"><span>LIVE / TEAM ROOM</span><span className="listener-count">● 42 listening</span></div>
					<p className="board-kicker">Quick temperature check</p>
					<h2>Are we ready to ship?</h2>
					<div className="demo-results">
						{demoAnswers.map((answer, index) => (
							<div className="demo-result" key={answer.label} style={{ '--result': `${answer.value}%`, '--delay': `${index * 90}ms` } as React.CSSProperties}>
								<div className="demo-fill" />
								<span>{answer.label}</span><strong>{answer.value}%</strong>
							</div>
						))}
					</div>
					<div className="arrival-tape" aria-hidden="true">
						<span>AM</span><span>RK</span><span>JS</span><span>+39</span><i>votes arriving</i>
					</div>
				</div>
			</section>

			<section className="principles page-wide" aria-labelledby="how-heading">
				<div className="section-intro">
					<p className="eyebrow">Three beats. One live moment.</p>
					<h2 id="how-heading">From question to collective signal.</h2>
				</div>
				<div className="principle-grid">
					<article><span>01</span><h3>Set the question</h3><p>Write two to ten clear choices. Add a close time when the decision needs a finish line.</p></article>
					<article><span>02</span><h3>Pass the mic</h3><p>Copy the link or put the QR on a screen. Guests vote in seconds—no account required.</p></article>
					<article><span>03</span><h3>Feel the shift</h3><p>Versioned results move live. If a connection blinks, the page quietly catches itself up.</p></article>
				</div>
			</section>

			<section className="engineering-note page-wide">
				<div><p className="eyebrow">Built for the awkward moments</p><h2>A vote is durable before the confetti.</h2></div>
				<p>MongoDB commits the ballot and event together. Redis handles the hot snapshot and fan-out. If live delivery pauses, the accepted vote remains safe and reconnect repairs the view.</p>
			</section>
		</main>
	)
}
