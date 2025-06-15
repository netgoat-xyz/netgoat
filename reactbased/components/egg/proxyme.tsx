'use client'
import confetti from 'canvas-confetti'
import { useEffect, useState } from 'react'

export default function ProxyMeConfetti() {
  const [typed, setTyped] = useState('')

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      setTyped(t => {
        const next = (t + e.key).slice(-7)
        if (next.toLowerCase() === 'proxyme') {
          confetti({ particleCount: 100, spread: 70, origin: { y: 0.6 } })
        }
        return next
      })
    }
    window.addEventListener('keypress', onKey)
    return () => window.removeEventListener('keypress', onKey)
  }, [])

  return null
}
