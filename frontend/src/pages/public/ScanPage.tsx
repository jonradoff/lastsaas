import { useState, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { motion } from 'framer-motion';

const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

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
    <div className="min-h-screen bg-slate-50 text-slate-700 flex flex-col">
      {/* Nav */}
      <header className="border-b border-slate-200 px-6 py-4 bg-white/80 backdrop-blur-lg">
        <div className="max-w-5xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-slate-900 text-lg">MCPLens</span>
          </Link>
          <Link
            to="/login"
            className="text-sm text-slate-500 hover:text-slate-900 transition-colors"
          >
            Sign in
          </Link>
        </div>
      </header>

      {/* Hero */}
      <main className="flex-1 flex items-center justify-center px-6 py-24">
        <motion.div
          initial={prefersReducedMotion ? false : { opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: [0.25, 0.46, 0.45, 0.94] }}
          className="max-w-2xl mx-auto text-center w-full"
        >
          <div className="inline-flex items-center gap-2 bg-blue-50 border border-blue-200 rounded-full px-4 py-1.5 text-sm text-blue-600 mb-8">
            AI Agent Readiness Scanner
          </div>

          <h1 className="text-4xl sm:text-5xl font-bold text-slate-900 leading-tight tracking-tight mb-6">
            How AI-ready is your Shopify store?
          </h1>

          <p className="text-lg text-slate-500 leading-relaxed mb-10">
            We connect to the store's MCP endpoint and test how well AI buyer agents can discover, evaluate, and purchase products.
          </p>

          <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-3 w-full max-w-xl mx-auto">
            <input
              type="text"
              value={domain}
              onChange={e => setDomain(e.target.value)}
              placeholder="Enter a Shopify store domain (e.g., allbirds.com)"
              className="flex-1 px-4 py-3 bg-white border border-slate-300 rounded-lg text-slate-900 text-sm placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
              autoFocus
            />
            <button
              type="submit"
              disabled={!domain.trim()}
              className="px-6 py-3 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold rounded-full transition-colors whitespace-nowrap"
            >
              Scan
            </button>
          </form>
        </motion.div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-200 px-6 py-6 bg-white">
        <div className="max-w-5xl mx-auto text-center text-sm text-slate-400">
          &copy; {new Date().getFullYear()} MCPLens
        </div>
      </footer>
    </div>
  );
}
