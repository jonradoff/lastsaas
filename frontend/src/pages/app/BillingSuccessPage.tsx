import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { CheckCircle } from 'lucide-react';

export default function BillingSuccessPage() {
  const navigate = useNavigate();

  useEffect(() => {
    const timer = setTimeout(() => navigate('/plan'), 3000);
    return () => clearTimeout(timer);
  }, [navigate]);

  return (
    <div className="flex flex-col items-center justify-center py-20">
      <CheckCircle className="w-16 h-16 text-accent-emerald mb-6" />
      <h1 className="text-2xl font-bold text-dark-50 mb-2">Payment Successful!</h1>
      <p className="text-dark-400">Redirecting to your plan...</p>
    </div>
  );
}
