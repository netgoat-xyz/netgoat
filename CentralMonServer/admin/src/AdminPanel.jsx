import React, { useState, useEffect } from 'react';
export default function AdminPanel(){
  const [token, setToken] = useState(localStorage.getItem('ng:token')||'');
  const [view, setView] = useState(token?'dashboard':'login');
  const [creds, setCreds] = useState({username:'',password:''});
  const [stats, setStats] = useState([]);
  const [history, setHistory] = useState([]);
  const [filters, setFilters] = useState({region:'',category:'',service:''});
  useEffect(()=>{ if(token){ fetchStats(); fetchHistory(); } },[token]);
  async function login(){
    const res = await fetch('/admin/login',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(creds)});
    const j = await res.json();
    if (j.token){ localStorage.setItem('ng:token', j.token); setToken(j.token); setView('dashboard'); }
  }
  async function fetchStats(){
    const q = new URLSearchParams(filters).toString();
    const r = await fetch('/api/stats?'+q); const j = await r.json(); setStats(j.reports||[]);
  }
  async function fetchHistory(){
    const q = new URLSearchParams(filters).toString();
    const r = await fetch('/api/history?'+q); const j = await r.json(); setHistory(j.history||[]);
  }
  if(view==='login'){
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50">
        <div className="w-full max-w-md p-6 bg-white rounded-2xl shadow">
          <h2 className="text-xl font-semibold mb-4">NetGoat Admin</h2>
          <input className="w-full mb-2 p-2 border rounded" placeholder="username" value={creds.username} onChange={e=>setCreds({...creds,username:e.target.value})} />
          <input className="w-full mb-4 p-2 border rounded" placeholder="password" type="password" value={creds.password} onChange={e=>setCreds({...creds,password:e.target.value})} />
          <button className="w-full py-2 rounded bg-sky-600 text-white" onClick={login}>Sign in</button>
        </div>
      </div>
    );
  }
  return (
    <div className="min-h-screen p-6 bg-slate-50">
      <header className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">NetGoat Admin</h1>
        <div className="flex gap-2">
          <button className="px-3 py-1 rounded border" onClick={()=>{ localStorage.removeItem('ng:token'); setToken(''); setView('login'); }}>Logout</button>
        </div>
      </header>
      <section className="grid grid-cols-3 gap-6">
        <div className="col-span-1 bg-white p-4 rounded shadow">
          <h3 className="font-semibold mb-2">Filters</h3>
          <select className="w-full mb-2 p-2 border" value={filters.region} onChange={e=>setFilters(f=>({...f,region:e.target.value}))}>
            <option value="">All regions</option>
            <option value="mm">mm</option>
            <option value="sg">sg</option>
            <option value="id">id</option>
          </select>
          <select className="w-full mb-2 p-2 border" value={filters.category} onChange={e=>setFilters(f=>({...f,category:e.target.value}))}>
            <option value="">All categories</option>
            <option value="main">main</option>
            <option value="logdb">logdb</option>
            <option value="sidecar">sidecar</option>
          </select>
          <input className="w-full mb-2 p-2 border" placeholder="service" value={filters.service} onChange={e=>setFilters(f=>({...f,service:e.target.value}))} />
          <button className="w-full py-2 rounded bg-sky-600 text-white" onClick={()=>{ fetchStats(); fetchHistory(); }}>Apply</button>
        </div>
        <div className="col-span-2 space-y-6">
          <div className="bg-white p-4 rounded shadow">
            <h3 className="font-semibold mb-2">Recent Reports</h3>
            <div className="overflow-auto max-h-72">
              <table className="w-full text-sm">
                <thead className="text-left text-xs text-slate-500"><tr><th>svc</th><th>region</th><th>cat</th><th>cpu%</th><th>seen</th></tr></thead>
                <tbody>
                  {stats.map((s,i)=> (
                    <tr key={i} className="border-t"><td>{s.service}</td><td>{s.regionId}</td><td>{s.category}</td><td>{s.stats?.appCpuUsagePercent?.toFixed?.(1)||'-'}</td><td>{new Date(s.receivedAt).toLocaleString()}</td></tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <div className="bg-white p-4 rounded shadow">
            <h3 className="font-semibold mb-2">History</h3>
            <div className="overflow-auto max-h-72 text-xs text-slate-700">
              {history.map((h,i)=> (
                <div key={i} className="py-1 border-b"><div className="font-medium">{h.status}</div><div className="text-xxs text-slate-500">{h.service||h.category} • {new Date(h.timestamp).toLocaleString()}</div></div>
              ))}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}