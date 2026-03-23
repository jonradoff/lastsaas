import { useEffect } from 'react';
import { Navigate, Link } from 'react-router-dom';
import DOMPurify from 'dompurify';
import { useBranding } from '../../contexts/BrandingContext';
import { useAuth } from '../../contexts/AuthContext';

export default function LandingPage() {
  const { branding, loaded } = useBranding();
  const { isAuthenticated } = useAuth();

  useEffect(() => {
    if (!loaded) return;
    const prevTitle = document.title;
    const title = branding.landingEnabled && branding.landingTitle
      ? branding.landingTitle
      : 'MCPLens — AI Agent Readiness Scanner';
    document.title = title;

    let metaDesc = document.querySelector('meta[name="description"]') as HTMLMetaElement | null;
    const meta = branding.landingEnabled && branding.landingMeta
      ? branding.landingMeta
      : 'MCPLens connects to any Shopify store\'s MCP endpoint, runs agent commerce scenarios, and scores how ready the store is for AI buyer agents.';
    if (meta) {
      if (!metaDesc) {
        metaDesc = document.createElement('meta');
        metaDesc.name = 'description';
        document.head.appendChild(metaDesc);
      }
      metaDesc.content = meta;
    }

    return () => {
      document.title = prevTitle;
      if (metaDesc) metaDesc.content = '';
    };
  }, [loaded, branding.landingEnabled, branding.landingTitle, branding.landingMeta]);

  // If user is logged in, go to dashboard
  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  if (!loaded) return null;

  // If custom landing HTML is configured, render it
  if (branding.landingEnabled && branding.landingHtml) {
    return (
      <div
        className="min-h-screen bg-dark-950"
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(branding.landingHtml) }}
      />
    );
  }

  // Default MCPLens landing page
  return (
    <div className="min-h-screen bg-dark-950 text-dark-200 flex flex-col">
      {/* Nav */}
      <header className="border-b border-dark-800 px-6 py-4">
        <div className="max-w-5xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-400 to-primary-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-white text-lg">MCPLens</span>
          </div>
          <Link
            to="/login"
            className="text-sm text-dark-400 hover:text-white transition-colors"
          >
            Sign in
          </Link>
        </div>
      </header>

      {/* Hero */}
      <main className="flex-1 flex items-center justify-center px-6 py-24">
        <div className="max-w-2xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 bg-primary-500/10 border border-primary-500/20 rounded-full px-4 py-1.5 text-sm text-primary-400 mb-8">
            AI-Native Commerce Intelligence
          </div>

          <h1 className="text-4xl sm:text-5xl font-bold text-white leading-tight mb-6">
            Scan any store's AI agent readiness
          </h1>

          <p className="text-lg text-dark-400 leading-relaxed mb-10">
            MCPLens connects to any Shopify store's MCP endpoint, runs agent commerce scenarios,
            and scores how ready the store is for AI buyer agents.
          </p>

          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link
              to="/signup"
              className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-primary-500 hover:bg-primary-400 text-white font-semibold rounded-lg transition-colors"
            >
              Scan a store free →
            </Link>
            <Link
              to="/login"
              className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-dark-800 hover:bg-dark-700 text-white font-medium rounded-lg border border-dark-700 transition-colors"
            >
              Sign in
            </Link>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-dark-800 px-6 py-6">
        <div className="max-w-5xl mx-auto text-center text-sm text-dark-500">
          © {new Date().getFullYear()} MCPLens
        </div>
      </footer>
    </div>
  );
}
