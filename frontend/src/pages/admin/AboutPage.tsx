import { useEffect, useState } from 'react';
import { Info } from 'lucide-react';
import { toast } from 'sonner';
import { adminApi } from '../../api/client';
import type { AboutInfo } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

export default function AboutPage() {
  const [about, setAbout] = useState<AboutInfo | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    adminApi.getAbout()
      .then(setAbout)
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-dark-50 flex items-center gap-3">
          <Info className="w-7 h-7 text-primary-400" />
          About
        </h1>
      </div>

      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-8">
        <div className="space-y-6">
          <div>
            <p className="text-sm text-dark-400 mb-1">Software</p>
            <p className="text-lg font-semibold text-dark-50">MCPLens</p>
          </div>
          <div>
            <p className="text-sm text-dark-400 mb-1">Version</p>
            <p className="text-lg font-semibold text-dark-50">{about?.version ?? 'Unknown'}</p>
          </div>
          <div>
            <p className="text-sm text-dark-400 mb-1">Copyright</p>
            <p className="text-dark-50">{about?.copyright}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
