import { Link } from 'react-router-dom';
import { XCircle } from 'lucide-react';

export default function BillingCancelPage() {
  return (
    <div className="flex flex-col items-center justify-center py-20">
      <XCircle className="w-16 h-16 text-dark-500 mb-6" />
      <h1 className="text-2xl font-bold text-dark-50 mb-2">Payment Canceled</h1>
      <p className="text-dark-400 mb-6">No charges were made.</p>
      <div className="flex gap-4">
        <Link
          to="/plan"
          className="px-4 py-2 text-sm bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
        >
          View Plans
        </Link>
        <Link
          to="/buy-credits"
          className="px-4 py-2 text-sm bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:border-dark-600 transition-colors"
        >
          Buy Credits
        </Link>
      </div>
    </div>
  );
}
