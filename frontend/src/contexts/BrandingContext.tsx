import { createContext, useContext, useEffect, useState } from 'react';
import type { BrandingConfig } from '../types';
import { brandingApi } from '../api/client';

const defaultBranding: BrandingConfig = {
  appName: 'MCPLens',
  tagline: '',
  logoMode: 'text',
  logoUrl: '',
  faviconUrl: '',
  primaryColor: '',
  accentColor: '',
  backgroundColor: '',
  surfaceColor: '',
  textColor: '',
  fontFamily: '',
  headingFont: '',
  landingEnabled: false,
  landingTitle: '',
  landingMeta: '',
  landingHtml: '',
  dashboardHtml: '',
  loginHeading: '',
  loginSubtext: '',
  signupHeading: '',
  signupSubtext: '',
  customCss: '',
  headHtml: '',
  ogImageUrl: '',
  navItems: [],
  analyticsSnippet: '',
};

interface BrandingContextValue {
  branding: BrandingConfig;
  loaded: boolean;
  reload: () => Promise<void>;
}

const BrandingContext = createContext<BrandingContextValue>({
  branding: defaultBranding,
  loaded: false,
  reload: async () => {},
});

export function BrandingProvider({ children }: { children: React.ReactNode }) {
  const [branding, setBranding] = useState<BrandingConfig>(defaultBranding);
  const [loaded, setLoaded] = useState(false);

  const load = async () => {
    try {
      const data = await brandingApi.get();
      setBranding(data);
    } catch {
      // Use defaults if branding endpoint fails
    } finally {
      setLoaded(true);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <BrandingContext.Provider value={{ branding, loaded, reload: load }}>
      {children}
    </BrandingContext.Provider>
  );
}

export function useBranding() {
  return useContext(BrandingContext);
}
