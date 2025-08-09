export default function GayDashboard() {
  return (
    <>
      <div className="w-full bg-gradient-to-r from-primary-600 via-primary-500 to-primary-400 text-white py-3 px-4 flex items-center justify-center shadow-lg z-50 fade-in border-b border-primary-700">
        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="tabler-icon tabler-icon-inner-shadow-top !size-5"><path d="M5.636 5.636a9 9 0 1 0 12.728 12.728a9 9 0 0 0 -12.728 -12.728z"></path><path d="M16.243 7.757a6 6 0 0 0 -8.486 0"></path></svg>
        <span className="font-semibold tracking-wide ml-1.5">NetGoat AlwaysAvaliable™</span>
        <span className="ml-2 text-white/80 text-sm">Your services stay available, even when the unexpected happens.</span>
      </div>
      <main className="min-h-screen bg-gradient-to-br from-zinc-900 to-zinc-800 text-zinc-100 flex flex-col items-center py-16 px-4">
        <div className="max-w-2xl w-full bg-zinc-900/80 rounded-2xl shadow-xl p-8 fade-in border border-zinc-800">
          <div className="flex items-center gap-4 mb-6">
            <img
              src="https://avatars.githubusercontent.com/u/583231?v=4"
              alt="Profile"
              className="w-20 h-20 rounded-full border-4 border-primary-500 shadow"
            />
            <div>
              <h1 className="text-3xl font-bold tracking-tight">John Doe</h1>
              <p className="text-zinc-400">
                Full Stack Developer &amp; Cloud Enthusiast
              </p>
            </div>
          </div>
          <p className="mb-6 text-zinc-300">
            Hi! I'm John, a passionate developer with experience in building
            scalable web apps, cloud infrastructure, and open source projects. I
            love working with TypeScript, React, Node.js, and all things cloud.
          </p>
          <div className="mb-6">
            <h2 className="text-xl font-semibold mb-2">Projects</h2>
            <ul className="space-y-2">
              <li className="bg-zinc-800 rounded-lg px-4 py-3 shadow hover:bg-zinc-700 transition">
                <a
                  href="https://github.com/Cloudable-dev/netgoat"
                  target="_blank"
                  rel="noopener"
                  className="font-semibold text-primary-400 hover:underline"
                >
                  NetGoat
                </a>
                <span className="block text-zinc-400 text-sm">
                  Open-source DNS &amp; cloud playground for learning and
                  experimentation.
                </span>
              </li>
              <li className="bg-zinc-800 rounded-lg px-4 py-3 shadow hover:bg-zinc-700 transition">
                <a
                  href="https://github.com/Cloudable-dev/elysia-stats"
                  target="_blank"
                  rel="noopener"
                  className="font-semibold text-primary-400 hover:underline"
                >
                  Elysia Stats
                </a>
                <span className="block text-zinc-400 text-sm">
                  Realtime server stats collection and monitoring with ElysiaJS.
                </span>
              </li>
            </ul>
          </div>
          <div className="mb-6">
            <h2 className="text-xl font-semibold mb-2">Contact</h2>
            <div className="flex gap-4">
              <a
                href="mailto:alexdoe@example.com"
                className="text-primary-400 hover:underline"
              >
                johndoe@example.com
              </a>
              <a
                href="https://github.com/Cloudable-dev"
                target="_blank"
                rel="noopener"
                className="text-primary-400 hover:underline"
              >
                GitHub
              </a>
            </div>
          </div>
        </div>
        <style>{`
        .fade-in { animation: fadeIn 0.7s cubic-bezier(.4,0,.2,1); }
        @keyframes fadeIn { from { opacity: 0; transform: translateY(16px);} to { opacity: 1; transform: none; } }
      `}</style>
      </main>
    </>
  );
}
