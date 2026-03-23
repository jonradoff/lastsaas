import { useNavigate } from 'react-router-dom';
import { Eye, X } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { setAuthToken } from '../api/client';

export default function ImpersonationBanner() {
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const isImpersonating = localStorage.getItem('mcplens_impersonating') === 'true';

  if (!isImpersonating || !user) return null;

  const endImpersonation = async () => {
    localStorage.removeItem('mcplens_impersonating');
    localStorage.removeItem('mcplens_access_token');
    localStorage.removeItem('mcplens_refresh_token');
    setAuthToken(null);
    await logout();
    navigate('/login');
  };

  return (
    <div className="fixed top-0 left-0 right-0 z-50 bg-amber-500 text-black px-4 py-2 flex items-center justify-center gap-3 text-sm font-medium">
      <Eye className="w-4 h-4" />
      <span>Viewing as <strong>{user.displayName}</strong> ({user.email})</span>
      <button
        onClick={endImpersonation}
        className="ml-2 inline-flex items-center gap-1 px-3 py-1 bg-black/20 hover:bg-black/30 rounded-lg text-xs font-medium transition-colors"
      >
        <X className="w-3 h-3" /> End Session
      </button>
    </div>
  );
}
