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
    <div className="bg-black text-white overflow-x-hidden">
      {/* 🧊 Hero */}
<section className="section-wrapper h-screen w-full flex flex-col justify-center items-center text-center px-6 bg-gradient-to-br from-black via-zinc-900 to-[#0d0d0d] relative overflow-hidden">
  <motion.h1
    initial={{ opacity: 0, y: 100 }}
    animate={{ opacity: 1, y: 0 }}
    transition={{ duration: 1 }}
    className="text-7xl md:text-8xl font-calsans font-black bg-gradient-to-tr from-fuchsia-300 via-indigo-400 to-white bg-clip-text text-transparent drop-shadow-[0_0_0.3rem_#ffffff88]"
  >
    Rewire The Internet.
  </motion.h1>
  <motion.p
    className="mt-6 max-w-2xl text-zinc-300 text-lg md:text-xl"
    initial={{ opacity: 0 }}
    animate={{ opacity: 1 }}
    transition={{ delay: 0.4 }}
  >
    A hyper-performant reverse proxy for modern developers. Powered by Bun. Inspired by chaos. Tuned for scale.
  </motion.p>
  <motion.div
    className="mt-10 flex gap-4"
    initial={{ opacity: 0 }}
    animate={{ opacity: 1 }}
    transition={{ delay: 0.6 }}
  >
    <Link href="/dashboard">
      <Button
        size="lg"
        className=" shadow-lg hover:scale-105 transition-transform"
      >
        Launch Now
      </Button>
    </Link>
    <Link href="/docs">
      <Button size="lg" variant="ghost" className="text-zinc-400 hover:text-white">
        Read the Docs
      </Button>
    </Link>
  </motion.div>

  {/* Floating Blurs (for that Apple depth glow) */}
    <div className="absolute bottom-[-200px] right-[-350px] w-[200px] h-[250px] bg-indigo-400 opacity-20 blur-3xl rounded-full" />
  <div className="absolute top-[-200px] left-[-200px] w-[400px] h-[400px] bg-fuchsia-500 opacity-20 blur-3xl rounded-full" />
  <div className="absolute bottom-[-200px] right-[-200px] w-[400px] h-[400px] bg-indigo-400 opacity-20 blur-3xl rounded-full" />
</section>

      {/* 🌈 Colored Feature Section */}
      <section className="py-32 px-6 bg-gradient-to-br from-[#141414] via-zinc-900 to-[#1e1e1e] text-center">
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
                className={`rounded-xl p-6 text-left bg-gradient-to-br ${f.color} text-black shadow-lg`}
              >
                <h3 className="text-2xl font-semibold mb-2">{f.title}</h3>
                <p className="text-sm text-black/80">{f.desc}</p>
              </motion.div>
            ))}
          </div>
        </FadeInSection>
      </section>

      {/* 🖼 Visual Product Section */}
      <section className="py-32 px-6 text-center bg-black">
        <FadeInSection>
          <h2 className="text-5xl font-semibold mb-12">Visual Monitoring</h2>
          <div className="flex flex-col md:flex-row items-center gap-12 max-w-6xl mx-auto">
            <img
              src="/proxy-dashboard-preview.png"
              alt="Dashboard preview"
              className="rounded-2xl shadow-2xl w-full md:w-1/2"
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
      <section className="py-32 px-6 bg-gradient-to-b from-black to-zinc-900 text-center">
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
                className="p-6 rounded-xl border border-zinc-700 bg-zinc-800 text-left"
              >
                <h4 className="text-xl font-medium">{item}</h4>
              </motion.div>
            ))}
          </div>
        </FadeInSection>
      </section>

      {/* 💬 Testimonials */}
      <section className="py-32 px-6 bg-gradient-to-t  from-black to-zinc-900 text-center">
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
                className="bg-zinc-800 p-6 rounded-xl text-left border border-zinc-700"
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
        <section className="py-32 px-6 text-center bg-black">
          <h2 className="text-5xl font-semibold mb-6">
            Proxy smarter. Not harder.
          </h2>
          <p className="text-zinc-400 mb-10 text-lg">
            Sign up free, deploy in minutes, scale globally.
          </p>
          <Link href="/signup">
            <Button size="lg">Launch Your Edge</Button>
          </Link>
        </section>
      </FadeInSection>
    </div>
  )
}
