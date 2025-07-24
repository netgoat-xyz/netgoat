'use client'

import { Button } from "@/components/ui/button"
import { motion, useInView } from "framer-motion"
import Link from "next/link"
import { useRef } from "react"

const FadeInSection = ({
  children,
  delay = 0,
}: {
  children: React.ReactNode
  delay?: number
}) => {
  const ref = useRef(null)
  const inView = useInView(ref, { once: true })

  return (
    <motion.div
      ref={ref}
      initial={{ opacity: 0, y: 80 }}
      animate={inView ? { opacity: 1, y: 0 } : {}}
      transition={{ duration: 0.8, delay }}
    >
      {children}
    </motion.div>
  )
}

export default function HomePage() {
  return (
    <div className="bg-gradient-to-br from-[#0a0a13] via-[#181825] to-[#1e1e2e] text-white overflow-x-hidden min-h-screen">
      {/* 🧊 Hero */}
<section className="section-wrapper h-screen w-full flex flex-col justify-center items-center text-center px-6 bg-gradient-to-br from-[#181825] via-[#232347] to-[#2e2e4d] relative overflow-hidden">
  <motion.h1
    initial={{ opacity: 0, y: 100, filter: 'blur(8px)' }}
    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
    transition={{ duration: 1, ease: 'easeOut' }}
    className="text-7xl md:text-8xl font-calsans font-black bg-gradient-to-tr from-fuchsia-400 via-indigo-400 to-cyan-200 bg-clip-text text-transparent drop-shadow-[0_0_0.5rem_#a5b4fc88]"
  >
    <span className="xl">Rewire The Internet.</span>
  </motion.h1>
  <motion.p
    className="mt-6 max-w-2xl text-zinc-200/90 text-lg md:text-xl backdrop-blur-md bg-white/5 rounded-xl px-4 py-2 shadow-lg"
    initial={{ opacity: 0, y: 30, filter: 'blur(8px)' }}
    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
    transition={{ delay: 0.4, duration: 0.8, ease: 'easeOut' }}
  >
    A hyper-performant reverse proxy for modern developers. <br /> <span className="text-fuchsia-300">Powered by Bun.</span> <span className="text-cyan-300">Inspired by chaos.</span> <span className="text-indigo-300">Tuned for scale.</span>
  </motion.p>
  <motion.div
    className="mt-10 flex gap-4"
    initial={{ opacity: 0, y: 30, filter: 'blur(8px)' }}
    animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
    transition={{ delay: 0.6, duration: 0.8, ease: 'easeOut' }}
  >
    <Link href="/dashboard">
      <Button
        size="lg"
        className="shadow-xl hover:scale-110 transition-transform bg-gradient-to-tr from-fuchsia-500 via-indigo-500 to-cyan-400 text-white border-0 backdrop-blur-xl"
        style={{ boxShadow: '0 4px 32px 0 #a5b4fc55' }}
      >
        Launch Now
      </Button>
    </Link>
    <Link href="/docs">
      <Button size="lg" variant="ghost" className="text-cyan-200 hover:text-white border border-cyan-400/30 bg-white/5 backdrop-blur-xl">
        Read the Docs
      </Button>
    </Link>
  </motion.div>

  {/* Floating Blurs (for that Apple depth glow) */}
  <div className="absolute bottom-[-200px] right-[-350px] w-[300px] h-[350px] bg-indigo-400/40 opacity-40 blur-3xl rounded-full" style={{filter:'blur(80px)'}} />
  <div className="absolute top-[-200px] left-[-200px] w-[400px] h-[400px] bg-fuchsia-500/40 opacity-40 blur-3xl rounded-full" style={{filter:'blur(80px)'}} />
  <div className="absolute bottom-[-200px] right-[-200px] w-[400px] h-[400px] bg-cyan-400/40 opacity-40 blur-3xl rounded-full" style={{filter:'blur(80px)'}} />
  <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-gradient-to-br from-fuchsia-400/20 via-indigo-400/20 to-cyan-400/20 rounded-full blur-3xl pointer-events-none" style={{filter:'blur(120px)'}} />
</section>

      {/* 🌈 Colored Feature Section */}
      <section className="py-32 px-6 bg-gradient-to-br from-[#181825] via-[#232347] to-[#2e2e4d] text-center">
        <FadeInSection>
          <h2 className="text-5xl font-bold mb-8">Seriously Smart Edge Routing</h2>
          <p className="text-zinc-400 text-lg max-w-2xl mx-auto">
            Our platform uses real-time heuristics and geolocation data to automatically route your requests to the fastest server—no config needed.
          </p>
          <div className="mt-16 grid grid-cols-1 md:grid-cols-3 gap-10 max-w-5xl mx-auto">
            {[
              {
                title: "Auto Scaling",
                desc: "Traffic surging? We auto-scale like Vercel, minus the bill shock.",
                color: "from-emerald-500 to-green-400"
              },
              {
                title: "Adaptive Caching",
                desc: "Your app. Our brain. Cached intelligently where it matters.",
                color: "from-blue-500 to-indigo-400"
              },
              {
                title: "Instant SSL",
                desc: "No more Let’s Encrypt weirdness. Just plug & secure.",
                color: "from-pink-500 to-rose-400"
              }
            ].map((f, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 40 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.2 + i * 0.2 }}
                className={`rounded-xl p-6 text-left bg-gradient-to-br ${f.color} text-black shadow-2xl backdrop-blur-xl/30`}
              >
                <h3 className="text-2xl font-semibold mb-2">{f.title}</h3>
                <p className="text-sm text-black/80">{f.desc}</p>
              </motion.div>
            ))}
          </div>
        </FadeInSection>
      </section>

      {/* 🖼 Visual Product Section */}
      <section className="py-32 px-6 text-center bg-black/90 backdrop-blur-2xl">
        <FadeInSection>
          <h2 className="text-5xl font-semibold mb-12">Visual Monitoring</h2>
          <div className="flex flex-col md:flex-row items-center gap-12 max-w-6xl mx-auto">
            <img
              src="/dashboard_demo.png"
              alt="Dashboard preview"
              className="rounded-2xl shadow-2xl w-full md:w-1/2 backdrop-blur-xl/30 border border-indigo-400/20"
              style={{boxShadow:'0 8px 64px 0 #a5b4fc33'}}
            />
            <div className="text-left max-w-md">
              <h3 className="text-2xl font-semibold mb-2">Live Logs & Stats</h3>
              <p className="text-zinc-400 mb-4">
                See every request, every edge node, every status code. In real-time. With filters, search, and export.
              </p>
              <h3 className="text-2xl font-semibold mb-2">Zero-Downtime Config Updates</h3>
              <p className="text-zinc-400">
                Change routing rules, auth, rate limits—instantly, with no service restarts.
              </p>
            </div>
          </div>
        </FadeInSection>
      </section>

      {/* 🧠 Advanced Features Section */}
      <section className="py-32 px-6 bg-[#232346] text-center">
        <FadeInSection>
          <h2 className="text-5xl font-bold mb-8">Built Different.</h2>
          <p className="text-zinc-400 mb-16 max-w-xl mx-auto text-lg">
            Unlike traditional proxies, we’re designed for chaos, scale, and modern app infrastructure. No NGINX files. No downtime. No limits.
          </p>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-10 max-w-6xl mx-auto">
            {[
              "Path + Domain Routing",
              "WebSocket Proxying",
              "JWT + IP Auth",
              "Rate Limiting",
              "Bun-Powered Performance",
              "DDoS Mitigation"
            ].map((item, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.1 }}
                className="p-6 rounded-xl border border-cyan-400/20 bg-zinc-800/80 text-left backdrop-blur-xl"
              >
                <h4 className="text-xl font-medium">{item}</h4>
              </motion.div>
            ))}
          </div>
        </FadeInSection>
      </section>

      {/* 💬 Testimonials */}
      <section className="py-32 px-6 bg-gradient-to-t from-[#181825] to-[#232347] text-center">
        <FadeInSection>
          <h2 className="text-4xl font-semibold mb-10">Trusted by Devs Worldwide 🌍</h2>
          <div className="grid md:grid-cols-2 gap-10 max-w-5xl mx-auto">
            {[
              {
                name: "Lena • DevOps Lead",
                quote: "We replaced 4 tools with this proxy. It’s fast, secure, and fun to work with."
              },
              {
                name: "Kazuki • Indie Hacker",
                quote: "Setup took 2 minutes. I was live with SSL, auth, and rate limits instantly."
              }
            ].map((t, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, y: 40 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.2 }}
                className="bg-zinc-800/80 p-6 rounded-xl text-left border border-fuchsia-400/20 backdrop-blur-xl shadow-xl"
              >
                <p className="text-lg text-zinc-300 italic">"{t.quote}"</p>
                <p className="mt-4 text-sm text-zinc-500">{t.name}</p>
              </motion.div>
            ))}
          </div>
        </FadeInSection>
      </section>

      {/* 🚀 Final CTA */}
      <FadeInSection delay={0.2}>
        <section className="py-32 px-6 text-center bg-gradient-to-br from-fuchsia-900/40 via-indigo-900/40 to-cyan-900/40 backdrop-blur-2xl border-t border-fuchsia-400/10 border-b border-cyan-400/10">
          <h2 className="text-5xl font-semibold mb-6 bg-gradient-to-tr from-fuchsia-400 via-indigo-400 to-cyan-200 bg-clip-text text-transparent drop-shadow-[0_0_0.5rem_#a5b4fc88]">Proxy smarter. Not harder.</h2>
          <p className="text-zinc-200/90 mb-10 text-lg backdrop-blur-md bg-white/5 rounded-xl px-4 py-2 shadow-lg inline-block">Sign up free, deploy in minutes, scale globally.</p>
          <Link href="/signup">
            <Button size="lg" className="shadow-xl hover:scale-110 transition-transform bg-gradient-to-tr from-fuchsia-500 via-indigo-500 to-cyan-400 text-white border-0 backdrop-blur-xl" style={{ boxShadow: '0 4px 32px 0 #a5b4fc55' }}>Launch Your Edge</Button>
          </Link>
        </section>
      </FadeInSection>

      {/* 💡 New: Community & Open Source Section */}
      <FadeInSection delay={0.3}>
        <section className="py-32 px-6 text-center bg-gradient-to-br from-cyan-900/40 via-indigo-900/40 to-fuchsia-900/40 backdrop-blur-2xl border-t border-cyan-400/10 border-b border-fuchsia-400/10">
          <h2 className="text-4xl font-semibold mb-6 bg-gradient-to-tr from-cyan-400 via-indigo-400 to-fuchsia-200 bg-clip-text text-transparent drop-shadow-[0_0_0.5rem_#a5b4fc88]">Open Source. Community Driven.</h2>
          <p className="text-zinc-200/90 mb-10 text-lg backdrop-blur-md bg-white/5 rounded-xl px-4 py-2 shadow-lg inline-block">Contribute, fork, or star us on GitHub. Join our Discord for support and memes.</p>
          <div className="flex flex-col md:flex-row gap-6 justify-center items-center">
            <a href="https://github.com/cloudable-dev/netgoat" target="_blank" rel="noopener" className="px-6 py-3 rounded-lg bg-gradient-to-tr from-indigo-500 to-cyan-400 text-white font-semibold shadow-lg hover:scale-105 transition-transform backdrop-blur-xl">GitHub</a>
            <a href="https://discord.gg/cloudable" target="_blank" rel="noopener" className="px-6 py-3 rounded-lg bg-gradient-to-tr from-fuchsia-500 to-indigo-400 text-white font-semibold shadow-lg hover:scale-105 transition-transform backdrop-blur-xl">Join Discord</a>
          </div>
        </section>
      </FadeInSection>
    </div>
  )
}
