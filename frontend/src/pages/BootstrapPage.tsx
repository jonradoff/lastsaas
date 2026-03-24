import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Terminal, RefreshCw } from 'lucide-react';
import { bootstrapApi } from '../api/client';

export default function BootstrapPage() {
  const navigate = useNavigate();
  const [checking, setChecking] = useState(false);

  const handleCheckAgain = async () => {
    setChecking(true);
    try {
      const data = await bootstrapApi.status();
      if (data.initialized) {
        navigate('/login');
      }
    } catch {
      // ignore
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center px-4">
      <div className="w-full max-w-lg">
        <div className="text-center mb-8">
          <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center mx-auto mb-4">
            <Terminal className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-white">System Setup Required</h1>
          <p className="text-dark-400 mt-2">
            Run the following command to create your initial admin account:
          </p>
        </div>

        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 space-y-5">
          <div className="bg-dark-950 border border-dark-700 rounded-lg p-4 font-mono text-sm">
            <div className="text-dark-500 mb-1">$</div>
            <div className="text-primary-400">cd backend && go run ./cmd/mcplens setup</div>
          </div>

          <p className="text-dark-400 text-sm">
            This will walk you through creating your organization and root administrator account.
            Once complete, click the button below to continue to login.
          </p>

          <div className="text-dark-500 text-xs space-y-1">
            <p>Other useful commands:</p>
            <p className="font-mono text-dark-400 ml-2">go run ./cmd/mcplens status</p>
            <p className="font-mono text-dark-400 ml-2">go run ./cmd/mcplens change-password --email you@example.com</p>
          </div>

          <button
            onClick={handleCheckAgain}
            disabled={checking}
            className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-60 disabled:cursor-not-allowed transition-all flex items-center justify-center gap-2"
          >
            <RefreshCw className={`w-4 h-4 ${checking ? 'animate-spin' : ''}`} />
            {checking ? 'Checking...' : 'Check Again'}
          </button>
        </div>
      </div>
    </div>
  );
}
