import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Gift, Download, Mail, Presentation, Wrench, Lock, Crown, ArrowRight } from 'lucide-react';
import { toast } from 'sonner';
import { plansApi } from '../../api/client';
import { getErrorMessage } from '../../utils/errors';
import { useTenant } from '../../contexts/TenantContext';
import { usePlan } from '../../contexts/PlanContext';
import { useTelemetry } from '../../hooks/useTelemetry';
import LoadingSpinner from '../../components/LoadingSpinner';
import type { Plan } from '../../types';

const TIER_ORDER: Record<string, number> = { free: 0, pro: 1, max: 2, agency: 3 };

interface BonusItem {
  id: string;
  title: string;
  description: string;
  detail: string;
  icon: React.ReactNode;
  fileName: string;
  downloadPath: string;
}

const BONUSES: BonusItem[] = [
  {
    id: 'cold-email-swipes',
    title: 'Cold Email Swipe File',
    description: '3 battle-tested cold email templates for agency outreach',
    detail:
      'White-label ready email templates covering initial outreach, follow-up with specific findings, and competitor comparison angles. Each includes subject line, body copy, P.S. line, and [PLACEHOLDER] tags for personalization.',
    icon: <Mail className="w-6 h-6" />,
    fileName: 'cold-email-swipes.md',
    downloadPath: '/bonuses/cold-email-swipes.md',
  },
  {
    id: 'agency-pitch-deck',
    title: 'Agency Pitch Deck',
    description: '10-slide white-label HTML pitch deck for client presentations',
    detail:
      'A self-contained HTML slide deck covering the AI commerce shift, your client\'s audit score, revenue impact calculator, prioritized roadmap, and a closing CTA. Includes [YOUR AGENCY NAME] and [CLIENT STORE] placeholders throughout.',
    icon: <Presentation className="w-6 h-6" />,
    fileName: 'agency-pitch-deck.html',
    downloadPath: '/bonuses/agency-pitch-deck.html',
  },
  {
    id: 'fix-snippets',
    title: 'Copy-Paste Fix Snippets',
    description: '10 common Shopify MCP issues with exact code fixes',
    detail:
      'Step-by-step remediation for the 10 most frequent MCP readiness gaps: missing prices, short descriptions, missing metafields, broken images, variant setup, slow response times, taxonomy, checkout API, inventory tracking, and search relevance.',
    icon: <Wrench className="w-6 h-6" />,
    fileName: 'fix-snippets.md',
    downloadPath: '/bonuses/fix-snippets.md',
  },
];

export default function BonusesPage() {
  const { activeTenant } = useTenant();
  const { trackPageView } = useTelemetry();
  const { currentPlan, plans, tierName, isLoaded } = usePlan();
  const navigate = useNavigate();

  useEffect(() => {
    trackPageView('/bonuses');
  }, [trackPageView]);

  if (!isLoaded) {
    return <LoadingSpinner size="lg" className="py-20" />;
  }

  const currentLevel = TIER_ORDER[tierName] ?? 0;

  // Fix snippets: Pro+ (level >= 1)
  // Agency bonuses (email swipe, pitch deck): Agency only (level >= 3)
  const hasFixSnippets = currentLevel >= 1;
  const hasAgencyBonuses = currentLevel >= 3;
  const hasAccess = hasFixSnippets; // Show page if any bonuses available

  const currentPlanName = currentPlan?.name ?? '';
  const agencyPlan = [...plans].sort((a, b) => b.monthlyPriceCents - a.monthlyPriceCents)[0] ?? null;

  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white flex items-center gap-3">
          <Gift className="w-7 h-7 text-primary-400" />
          Agency Bonus Content
        </h1>
        <p className="text-dark-400 mt-1">
          Exclusive materials to help you sell and deliver AI commerce readiness audits
        </p>
      </div>

      {/* Access gate */}
      {!hasAccess ? (
        <UpgradePrompt currentPlanName={currentPlanName} agencyPlan={agencyPlan} onNavigate={() => navigate('/plan')} />
      ) : (
        <>
          {/* Access level banner */}
          {hasAgencyBonuses ? (
            <div className="bg-gradient-to-r from-primary-500/10 via-accent-purple/10 to-primary-500/10 border border-primary-500/20 rounded-2xl p-5 mb-8 flex items-center gap-4">
              <Crown className="w-6 h-6 text-primary-400 flex-shrink-0" />
              <div>
                <p className="text-sm font-semibold text-primary-300">Agency Tier — Full Access</p>
                <p className="text-sm text-dark-400 mt-0.5">
                  All bonus materials are available for download. White-label and redistribute freely to clients.
                </p>
              </div>
            </div>
          ) : (
            <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-5 mb-8 flex items-center gap-4">
              <Gift className="w-6 h-6 text-primary-400 flex-shrink-0" />
              <div>
                <p className="text-sm font-semibold text-dark-200">Pro Tier Bonuses</p>
                <p className="text-sm text-dark-400 mt-0.5">
                  Fix snippets are available. Upgrade to Agency for the full bonus suite including email templates and pitch deck.
                </p>
              </div>
            </div>
          )}

          {/* Bonus cards */}
          <div className="grid gap-6 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
            {BONUSES.map((bonus) => {
              const isAgencyOnly = bonus.id === 'cold-email-swipes' || bonus.id === 'agency-pitch-deck';
              if (isAgencyOnly && !hasAgencyBonuses) {
                return <LockedBonusCard key={bonus.id} bonus={bonus} onUpgrade={() => navigate('/plan')} />;
              }
              return <BonusCard key={bonus.id} bonus={bonus} />;
            })}
          </div>

          {/* Footer note */}
          <p className="text-dark-500 text-sm mt-8 text-center">
            Files are served from this app's public directory. Right-click &rarr; Save As to download, or open in a new tab.
          </p>
        </>
      )}
    </div>
  );
}

function BonusCard({ bonus }: { bonus: BonusItem }) {
  return (
    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 flex flex-col hover:border-dark-700 transition-colors">
      {/* Icon */}
      <div className="w-12 h-12 rounded-xl bg-primary-500/10 border border-primary-500/20 flex items-center justify-center text-primary-400 mb-4 flex-shrink-0">
        {bonus.icon}
      </div>

      {/* Content */}
      <h3 className="text-base font-semibold text-white mb-1">{bonus.title}</h3>
      <p className="text-sm font-medium text-dark-300 mb-3">{bonus.description}</p>
      <p className="text-sm text-dark-400 leading-relaxed flex-1">{bonus.detail}</p>

      {/* Download button */}
      <a
        href={bonus.downloadPath}
        download={bonus.fileName}
        target="_blank"
        rel="noreferrer"
        className="mt-6 w-full flex items-center justify-center gap-2 py-2.5 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
      >
        <Download className="w-4 h-4" />
        Download {bonus.fileName}
      </a>
    </div>
  );
}

function LockedBonusCard({ bonus, onUpgrade }: { bonus: BonusItem; onUpgrade: () => void }) {
  return (
    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6 flex flex-col opacity-60">
      <div className="w-12 h-12 rounded-xl bg-dark-800 border border-dark-700 flex items-center justify-center text-dark-500 mb-4 flex-shrink-0">
        {bonus.icon}
      </div>
      <h3 className="text-base font-semibold text-dark-300 mb-1">{bonus.title}</h3>
      <p className="text-sm text-dark-500 mb-3">{bonus.description}</p>
      <p className="text-sm text-dark-500 leading-relaxed flex-1">{bonus.detail}</p>
      <button
        onClick={onUpgrade}
        className="mt-6 w-full flex items-center justify-center gap-2 py-2.5 text-sm font-medium bg-dark-800 text-dark-400 rounded-lg hover:bg-dark-700 transition-colors"
      >
        <Lock className="w-4 h-4" />
        Upgrade to Agency
      </button>
    </div>
  );
}

interface UpgradePromptProps {
  currentPlanName: string;
  agencyPlan: Plan | null;
  onNavigate: () => void;
}

function UpgradePrompt({ currentPlanName, agencyPlan, onNavigate }: UpgradePromptProps) {
  return (
    <div className="rounded-2xl border border-dark-700 bg-dark-900/50 p-10 flex flex-col items-center text-center">
      {/* Lock icon */}
      <div className="w-16 h-16 rounded-full bg-dark-800 border border-dark-700 flex items-center justify-center mb-6">
        <Lock className="w-7 h-7 text-dark-400" />
      </div>

      <h2 className="text-xl font-bold text-white mb-2">Agency Plan Required</h2>
      <p className="text-dark-300 text-sm max-w-md mb-1">
        These bonus materials are exclusive to the Agency tier.
        {currentPlanName && (
          <>
            {' '}You're currently on the{' '}
            <span className="text-white font-medium">{currentPlanName}</span> plan.
          </>
        )}
      </p>
      <p className="text-dark-400 text-sm max-w-md mb-8">
        Upgrade to get the cold email swipe file, white-label pitch deck, and copy-paste fix snippets — plus all other Agency features.
      </p>

      {/* What's included preview */}
      <div className="w-full max-w-md bg-dark-800/50 border border-dark-700 rounded-xl p-5 mb-8 text-left">
        <p className="text-xs font-semibold text-dark-400 uppercase tracking-wider mb-3">
          Included with Agency
        </p>
        <div className="space-y-3">
          {BONUSES.map((bonus) => (
            <div key={bonus.id} className="flex items-start gap-3">
              <div className="w-7 h-7 rounded-lg bg-dark-700 flex items-center justify-center text-dark-300 flex-shrink-0 mt-0.5">
                {bonus.icon}
              </div>
              <div>
                <p className="text-sm font-medium text-dark-200">{bonus.title}</p>
                <p className="text-xs text-dark-500">{bonus.description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <button
        onClick={onNavigate}
        className="flex items-center gap-2 px-6 py-3 bg-primary-500 text-white rounded-xl font-medium text-sm hover:bg-primary-600 transition-colors"
      >
        {agencyPlan ? `Upgrade to ${agencyPlan.name}` : 'View Plans'}
        <ArrowRight className="w-4 h-4" />
      </button>
    </div>
  );
}
