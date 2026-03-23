/**
 * MCPLens Logo
 * "MCP" in a 3D blue rounded box + "Lens" in a frosted glass style
 */
export default function Logo({ size = 'default' }: { size?: 'small' | 'default' | 'large' }) {
  const scales = {
    small: { box: 'w-7 h-7', boxText: 'text-[10px]', lens: 'text-base', gap: 'gap-1.5' },
    default: { box: 'w-9 h-9', boxText: 'text-xs', lens: 'text-xl', gap: 'gap-2' },
    large: { box: 'w-12 h-12', boxText: 'text-sm', lens: 'text-3xl', gap: 'gap-2.5' },
  };
  const s = scales[size];

  return (
    <div className={`flex items-center ${s.gap}`}>
      {/* MCP — 3D blue box */}
      <div
        className={`${s.box} rounded-xl flex items-center justify-center relative`}
        style={{
          background: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 50%, #1d4ed8 100%)',
          boxShadow: '0 4px 12px rgba(37, 99, 235, 0.4), 0 2px 4px rgba(37, 99, 235, 0.2), inset 0 1px 0 rgba(255,255,255,0.2)',
        }}
      >
        <span
          className={`${s.boxText} font-bold tracking-tight text-white`}
          style={{ textShadow: '0 1px 2px rgba(0,0,0,0.2)' }}
        >
          MCP
        </span>
      </div>

      {/* Lens — frosted glass text */}
      <span
        className={`${s.lens} font-semibold tracking-tight`}
        style={{
          background: 'linear-gradient(180deg, rgba(15,23,42,0.85) 0%, rgba(15,23,42,0.55) 100%)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          backgroundClip: 'text',
        }}
      >
        Lens
      </span>
    </div>
  );
}

/** Inline variant for tight spaces (single line, no flex) */
export function LogoInline({ className = '' }: { className?: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 ${className}`}>
      <span
        className="inline-flex items-center justify-center w-6 h-6 rounded-lg text-[9px] font-bold text-white tracking-tight"
        style={{
          background: 'linear-gradient(135deg, #3b82f6, #1d4ed8)',
          boxShadow: '0 2px 6px rgba(37,99,235,0.3), inset 0 1px 0 rgba(255,255,255,0.15)',
          textShadow: '0 1px 1px rgba(0,0,0,0.15)',
        }}
      >
        MCP
      </span>
      <span
        className="text-lg font-semibold tracking-tight"
        style={{
          background: 'linear-gradient(180deg, rgba(15,23,42,0.85), rgba(15,23,42,0.55))',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          backgroundClip: 'text',
        }}
      >
        Lens
      </span>
    </span>
  );
}
