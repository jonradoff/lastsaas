import { Link } from 'react-router-dom';
import { Lock } from 'lucide-react';
import { usePlan } from '../contexts/PlanContext';

interface UpgradeGateProps {
  /** The entitlement key to check (e.g., "ai_assessment", "team_access") */
  entitlement?: string;
  /** Or check by minimum tier name */
  minTier?: 'pro' | 'max' | 'agency';
  /** What to show when the user HAS access */
  children: React.ReactNode;
  /** Feature name for the upgrade prompt */
  featureName: string;
  /** Which tier unlocks this feature */
  requiredTier?: string;
  /** Optional: render a blurred preview instead of full lock */
  preview?: boolean;
}

const TIER_ORDER: Record<string, number> = { free: 0, pro: 1, max: 2, agency: 3 };

export default function UpgradeGate({
  entitlement,
  minTier,
  children,
  featureName,
  requiredTier,
  preview = false,
}: UpgradeGateProps) {
  const { hasEntitlement, tierName, isLoaded } = usePlan();

  // Don't gate until plan data has loaded
  if (!isLoaded) return null;

  let hasAccess = true;

  if (entitlement) {
    hasAccess = hasEntitlement(entitlement);
  }

  if (minTier) {
    const currentLevel = TIER_ORDER[tierName] ?? 0;
    const requiredLevel = TIER_ORDER[minTier] ?? 0;
    hasAccess = currentLevel >= requiredLevel;
  }

  if (hasAccess) {
    return <>{children}</>;
  }

  const tierLabel = requiredTier ?? minTier ?? 'a higher plan';

  if (preview) {
    return (
      <div className="relative">
        <div className="blur-sm pointer-events-none select-none" aria-hidden>
          {children}
        </div>
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="bg-dark-900/90 border border-dark-700 rounded-xl p-6 text-center max-w-sm">
            <Lock className="w-8 h-8 text-dark-500 mx-auto mb-3" />
            <h3 className="text-dark-50 font-semibold mb-1">{featureName}</h3>
            <p className="text-dark-400 text-sm mb-4">
              Upgrade to {tierLabel} to unlock this feature.
            </p>
            <Link
              to="/plan"
              className="inline-flex items-center px-4 py-2 bg-primary-500 hover:bg-primary-600 text-white text-sm font-medium rounded-lg transition-colors"
            >
              View Plans
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-dark-900 border border-dark-700 rounded-xl p-8 text-center">
      <Lock className="w-10 h-10 text-dark-500 mx-auto mb-4" />
      <h3 className="text-lg font-semibold text-dark-50 mb-2">{featureName}</h3>
      <p className="text-dark-400 text-sm mb-6 max-w-md mx-auto">
        This feature is available on the {tierLabel} plan and above.
      </p>
      <Link
        to="/plan"
        className="inline-flex items-center px-5 py-2.5 bg-primary-500 hover:bg-primary-600 text-white font-semibold rounded-lg transition-colors"
      >
        Upgrade to {tierLabel}
      </Link>
    </div>
  );
}
