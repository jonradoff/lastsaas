import { useState, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';

export default function ScanPage() {
  const [domain, setDomain] = useState('');
  const navigate = useNavigate();

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = domain.trim().replace(/^https?:\/\//, '').replace(/\/$/, '');
    if (!trimmed) return;
    navigate(`/scan/${trimmed}`);
  }

  return (
    <div className="min-h-screen bg-[#0f172a] text-slate-200 flex flex-col">
      {/* Nav */}
      <header className="border-b border-slate-800 px-6 py-4">
        <div className="max-w-5xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-sky-400 to-sky-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-white text-lg">MCPLens</span>
          </Link>
          <Link
            to="/login"
            className="text-sm text-slate-400 hover:text-white transition-colors"
          >
            Sign in
          </Link>
        </div>
      </header>

      {/* Hero */}
      <main className="flex-1 flex items-center justify-center px-6 py-24">
        <div className="max-w-2xl mx-auto text-center w-full">
          <div className="inline-flex items-center gap-2 bg-sky-500/10 border border-sky-500/20 rounded-full px-4 py-1.5 text-sm text-sky-400 mb-8">
            AI Agent Readiness Scanner
          </div>

          <h1 className="text-4xl sm:text-5xl font-bold text-white leading-tight mb-6">
            How AI-ready is your Shopify store?
          </h1>

          <p className="text-lg text-slate-400 leading-relaxed mb-10">
            We connect to the store's MCP endpoint and test how well AI buyer agents can discover, evaluate, and purchase products.
          </p>

          <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-3 w-full max-w-xl mx-auto">
            <input
              type="text"
              value={domain}
              onChange={e => setDomain(e.target.value)}
              placeholder="Enter a Shopify store domain (e.g., allbirds.com)"
              className="flex-1 px-4 py-3 bg-[#1e293b] border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:outline-none focus:border-sky-500 transition-colors"
              autoFocus
            />
            <button
              type="submit"
              disabled={!domain.trim()}
              className="px-6 py-3 bg-sky-500 hover:bg-sky-400 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-colors whitespace-nowrap"
            >
              Scan
            </button>
          </form>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 px-6 py-6">
        <div className="max-w-5xl mx-auto text-center text-sm text-slate-500">
          © {new Date().getFullYear()} MCPLens
        </div>
      </footer>
    </div>
  );
}
