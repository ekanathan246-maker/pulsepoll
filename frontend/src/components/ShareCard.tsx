import { useEffect, useRef, useState } from 'react'
import QRCode from 'qrcode'
import { shareUrl } from '../lib/api'

export default function ShareCard({ slug }: { slug: string }) {
  const url = shareUrl(slug)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!canvasRef.current) return
    QRCode.toCanvas(canvasRef.current, url, {
      width: 600,
      margin: 0,
      errorCorrectionLevel: 'M',
      color: { dark: '#160203', light: '#ffffff' },
    }).catch(() => {})
  }, [url])

  async function copy() {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      window.prompt('Copy this link:', url)
    }
  }

  async function nativeShare() {
    try {
      await navigator.share({ title: 'PulsePoll', url })
    } catch {
      /* user dismissed */
    }
  }

  async function downloadQr() {
    const dataUrl = await QRCode.toDataURL(url, {
      width: 1024,
      margin: 0,
      errorCorrectionLevel: 'M',
      color: { dark: '#160203', light: '#ffffff' },
    })
    const a = document.createElement('a')
    a.href = dataUrl
    a.download = `poll-${slug}-qr.png`
    a.click()
  }

  return (
    <div className="share-card">
      <div className="share-card-head">
        <div>
          <div className="share-card-title">Share this poll</div>
          <div className="share-card-sub">Copy the link, share it, or let people scan the QR code.</div>
        </div>
        <span className="share-chip">Free to vote</span>
      </div>

      <div className="share-input-row">
        <input className="input share-input" readOnly value={url} onFocus={(e) => e.currentTarget.select()} />
        <button className="btn" onClick={copy}>
          {copied ? 'Copied ✓' : 'Copy link'}
        </button>
      </div>

      <div className="share-qr-row">
        <div className="qr-frame" title="Scan to open this poll">
          <canvas ref={canvasRef} className="qr-canvas" />
        </div>
        <div className="share-qr-side">
          <p className="qr-hint">
            Scan the code with any phone camera to open this poll instantly.
          </p>
          <div className="share-btn-grid">
            <button className="btn" onClick={downloadQr}>
              Download QR
            </button>
            {typeof navigator.share === 'function' && (
              <button className="btn btn-primary" onClick={nativeShare}>
                Share…
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}