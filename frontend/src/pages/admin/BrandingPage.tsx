import { useEffect, useState, useRef } from 'react';
import { Paintbrush, Upload, X, Check, Plus, Trash2, GripVertical, Eye, EyeOff, FileText, Image, Globe } from 'lucide-react';
import { toast } from 'sonner';
import { brandingApi, brandingAdminApi } from '../../api/client';
import { useBranding } from '../../contexts/BrandingContext';
import { useTenant } from '../../contexts/TenantContext';
import type { BrandingConfig, NavItem, MediaItem, CustomPage } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';
import { getErrorMessage } from '../../utils/errors';

type Tab = 'identity' | 'theme' | 'content' | 'pages' | 'media';

export default function BrandingPage() {
  const { reload } = useBranding();
  const { role } = useTenant();
  const isOwner = role === 'owner';
  const [tab, setTab] = useState<Tab>('identity');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [config, setConfig] = useState<BrandingConfig | null>(null);

  // Media state
  const [media, setMedia] = useState<MediaItem[]>([]);
  const [mediaLoading, setMediaLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const mediaInputRef = useRef<HTMLInputElement>(null);

  // Pages state
  const [pages, setPages] = useState<CustomPage[]>([]);
  const [pagesLoading, setPagesLoading] = useState(false);
  const [editingPage, setEditingPage] = useState<Partial<CustomPage> | null>(null);
  const [pageSaving, setPageSaving] = useState(false);

  useEffect(() => {
    brandingApi.get()
      .then((data) => setConfig(data))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (tab === 'media') loadMedia();
    if (tab === 'pages') loadPages();
  }, [tab]);

  const loadMedia = () => {
    setMediaLoading(true);
    brandingAdminApi.listMedia()
      .then((data) => setMedia(data.media))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setMediaLoading(false));
  };

  const loadPages = () => {
    setPagesLoading(true);
    brandingAdminApi.listPages()
      .then((data) => setPages(data.pages))
      .catch(err => toast.error(getErrorMessage(err)))
      .finally(() => setPagesLoading(false));
  };

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await brandingAdminApi.update(config);
      await reload();
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch {
      // error
    } finally {
      setSaving(false);
    }
  };

  const handleAssetUpload = async (key: 'logo' | 'favicon', file: File) => {
    try {
      await brandingAdminApi.uploadAsset(key, file);
      // Refresh branding to get new URLs
      const data = await brandingApi.get();
      setConfig(data);
      await reload();
    } catch {
      // error
    }
  };

  const handleAssetDelete = async (key: 'logo' | 'favicon') => {
    try {
      await brandingAdminApi.deleteAsset(key);
      const data = await brandingApi.get();
      setConfig(data);
      await reload();
    } catch {
      // error
    }
  };

  const handleMediaUpload = async (file: File) => {
    setUploading(true);
    try {
      await brandingAdminApi.uploadMedia(file);
      loadMedia();
    } catch {
      // error
    } finally {
      setUploading(false);
    }
  };

  const handleMediaDelete = async (key: string) => {
    try {
      await brandingAdminApi.deleteMedia(key);
      setMedia(prev => prev.filter(m => m.key !== key));
    } catch {
      // error
    }
  };

  const handlePageSave = async () => {
    if (!editingPage) return;
    setPageSaving(true);
    try {
      if (editingPage.id) {
        await brandingAdminApi.updatePage(editingPage.id, editingPage);
      } else {
        await brandingAdminApi.createPage(editingPage);
      }
      setEditingPage(null);
      loadPages();
      await reload();
    } catch {
      // error
    } finally {
      setPageSaving(false);
    }
  };

  const handlePageDelete = async (id: string) => {
    try {
      await brandingAdminApi.deletePage(id);
      setPages(prev => prev.filter(p => p.id !== id));
    } catch {
      // error
    }
  };

  const update = (field: string, value: unknown) => {
    setConfig(prev => prev ? { ...prev, [field]: value } : prev);
  };

  const updateNavItem = (index: number, field: keyof NavItem, value: unknown) => {
    setConfig(prev => {
      if (!prev) return prev;
      const items = [...prev.navItems];
      items[index] = { ...items[index], [field]: value };
      return { ...prev, navItems: items };
    });
  };

  const addNavItem = () => {
    setConfig(prev => {
      if (!prev) return prev;
      const newItem: NavItem = {
        id: `custom_${Date.now()}`,
        label: 'New Page',
        icon: 'FileText',
        target: '/p/',
        isBuiltIn: false,
        visible: true,
        sortOrder: prev.navItems.length,
      };
      return { ...prev, navItems: [...prev.navItems, newItem] };
    });
  };

  const removeNavItem = (index: number) => {
    setConfig(prev => {
      if (!prev) return prev;
      const items = prev.navItems.filter((_, i) => i !== index);
      return { ...prev, navItems: items };
    });
  };

  if (loading) return <LoadingSpinner size="lg" className="py-20" />;
  if (!config) return <div className="text-dark-400 py-20 text-center">Failed to load branding config.</div>;

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'identity', label: 'Identity', icon: <Globe className="w-4 h-4" /> },
    { key: 'theme', label: 'Theme', icon: <Paintbrush className="w-4 h-4" /> },
    { key: 'content', label: 'Content', icon: <FileText className="w-4 h-4" /> },
    { key: 'pages', label: 'Pages', icon: <FileText className="w-4 h-4" /> },
    { key: 'media', label: 'Media', icon: <Image className="w-4 h-4" /> },
  ];

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Paintbrush className="w-7 h-7 text-accent-pink" />
            Branding
          </h1>
          <p className="text-dark-400 mt-1">Customize the look and feel of your app</p>
        </div>
        {isOwner && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors text-sm font-medium"
          >
            {saved ? <><Check className="w-4 h-4" /> Saved</> : saving ? 'Saving...' : 'Save Changes'}
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 bg-dark-900/50 border border-dark-800 rounded-xl p-1">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              tab === t.key ? 'bg-primary-500/20 text-primary-400' : 'text-dark-400 hover:text-white hover:bg-dark-800/50'
            }`}
          >
            {t.icon}
            {t.label}
          </button>
        ))}
      </div>

      {/* Identity Tab */}
      {tab === 'identity' && (
        <div className="space-y-6">
          <Section title="App Identity">
            <Field label="App Name" description="Replaces 'MCPLens' everywhere in the app">
              <input value={config.appName} onChange={e => update('appName', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="My App" />
            </Field>
            <Field label="Tagline">
              <input value={config.tagline} onChange={e => update('tagline', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Your tagline here" />
            </Field>
          </Section>

          <Section title="Logo">
            <Field label="Logo Mode" description="How the logo is displayed in the header">
              <select value={config.logoMode} onChange={e => update('logoMode', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner}>
                <option value="text">Text only (app name)</option>
                <option value="image">Image only</option>
                <option value="both">Image + text</option>
              </select>
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <AssetUploader
                label="Logo Image"
                currentUrl={config.logoUrl}
                onUpload={(f) => handleAssetUpload('logo', f)}
                onDelete={() => handleAssetDelete('logo')}
                isOwner={isOwner}
              />
              <AssetUploader
                label="Favicon"
                currentUrl={config.faviconUrl}
                onUpload={(f) => handleAssetUpload('favicon', f)}
                onDelete={() => handleAssetDelete('favicon')}
                isOwner={isOwner}
              />
            </div>
          </Section>

          <Section title="Auth Pages">
            <div className="grid grid-cols-2 gap-4">
              <Field label="Login Heading">
                <input value={config.loginHeading} onChange={e => update('loginHeading', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Welcome back" />
              </Field>
              <Field label="Login Subtext">
                <input value={config.loginSubtext} onChange={e => update('loginSubtext', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Sign in to your account" />
              </Field>
              <Field label="Signup Heading">
                <input value={config.signupHeading} onChange={e => update('signupHeading', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Create your account" />
              </Field>
              <Field label="Signup Subtext">
                <input value={config.signupSubtext} onChange={e => update('signupSubtext', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Get started" />
              </Field>
            </div>
          </Section>
        </div>
      )}

      {/* Theme Tab */}
      {tab === 'theme' && (
        <div className="space-y-6">
          <Section title="Colors" description="Set your brand colors. The primary color generates a full shade palette automatically.">
            <div className="grid grid-cols-2 gap-4">
              <ColorField label="Primary Color" value={config.primaryColor} onChange={v => update('primaryColor', v)} isOwner={isOwner} />
              <ColorField label="Accent Color" value={config.accentColor} onChange={v => update('accentColor', v)} isOwner={isOwner} />
            </div>
          </Section>

          <Section title="Typography">
            <div className="grid grid-cols-2 gap-4">
              <Field label="Body Font Family" description="Google Fonts name (e.g., 'Inter', 'Roboto')">
                <input value={config.fontFamily} onChange={e => update('fontFamily', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Inter" />
              </Field>
              <Field label="Heading Font Family">
                <input value={config.headingFont} onChange={e => update('headingFont', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Same as body" />
              </Field>
            </div>
          </Section>

          <Section title="Custom CSS" description="Injected after theme variables. Power users can fine-tune anything.">
            <textarea
              value={config.customCss}
              onChange={e => update('customCss', e.target.value)}
              className={`${isOwner ? inputClass : disabledInputClass} font-mono text-xs h-40`}
              disabled={!isOwner}
              placeholder="/* Custom CSS */"
            />
          </Section>

          <Section title="Custom <head> HTML" description="Additional HTML injected into the document head (meta tags, scripts, etc.)">
            <textarea
              value={config.headHtml}
              onChange={e => update('headHtml', e.target.value)}
              className={`${isOwner ? inputClass : disabledInputClass} font-mono text-xs h-32`}
              disabled={!isOwner}
              placeholder="<!-- Custom head HTML -->"
            />
          </Section>
        </div>
      )}

      {/* Content Tab */}
      {tab === 'content' && (
        <div className="space-y-6">
          <Section title="Landing Page" description="Public page at / for unauthenticated visitors. When disabled, visitors are redirected to the login page.">
            <Field label="Enable Landing Page">
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={config.landingEnabled} onChange={e => update('landingEnabled', e.target.checked)} className="rounded" disabled={!isOwner} />
                <span className="text-sm text-dark-300">Show landing page instead of redirecting to login</span>
              </label>
            </Field>
            {config.landingEnabled && (
              <>
                <div className="grid grid-cols-2 gap-4">
                  <Field label="Page Title (SEO)">
                    <input value={config.landingTitle} onChange={e => update('landingTitle', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="My App - Get Started" />
                  </Field>
                  <Field label="Meta Description (SEO)">
                    <input value={config.landingMeta} onChange={e => update('landingMeta', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="Description for search engines" />
                  </Field>
                </div>
                <Field label="Landing Page HTML">
                  <textarea
                    value={config.landingHtml}
                    onChange={e => update('landingHtml', e.target.value)}
                    className={`${isOwner ? inputClass : disabledInputClass} font-mono text-xs h-64`}
                    disabled={!isOwner}
                    placeholder="<div>Your landing page HTML here...</div>"
                  />
                </Field>
              </>
            )}
          </Section>

          <Section title="Dashboard HTML" description="Custom HTML block shown on the user dashboard.">
            <textarea
              value={config.dashboardHtml}
              onChange={e => update('dashboardHtml', e.target.value)}
              className={`${isOwner ? inputClass : disabledInputClass} font-mono text-xs h-40`}
              disabled={!isOwner}
              placeholder="<div>Welcome message, announcements, etc.</div>"
            />
          </Section>

          <Section title="Open Graph" description="Default social sharing image.">
            <Field label="OG Image URL">
              <input value={config.ogImageUrl} onChange={e => update('ogImageUrl', e.target.value)} className={isOwner ? inputClass : disabledInputClass} disabled={!isOwner} placeholder="https://..." />
            </Field>
          </Section>

          <Section title="Navigation" description="Configure which items appear in the app sidebar. Built-in items can be hidden but not removed. Custom items link to your custom pages.">
            <div className="space-y-2">
              {config.navItems.map((item, i) => (
                <div key={item.id} className="flex items-center gap-3 p-3 bg-dark-800/50 border border-dark-700/50 rounded-lg">
                  <GripVertical className="w-4 h-4 text-dark-600 shrink-0" />
                  <input
                    value={item.label}
                    onChange={e => updateNavItem(i, 'label', e.target.value)}
                    className={`w-32 px-2 py-1 border rounded text-sm ${!isOwner || item.isBuiltIn ? 'bg-dark-800/50 border-dark-700/50 text-dark-400 cursor-not-allowed' : 'bg-dark-800 border-dark-700 text-white'}`}
                    disabled={!isOwner || item.isBuiltIn}
                  />
                  <select
                    value={item.icon}
                    onChange={e => updateNavItem(i, 'icon', e.target.value)}
                    className={`w-40 px-2 py-1 border rounded text-sm ${!isOwner ? 'bg-dark-800/50 border-dark-700/50 text-dark-400 cursor-not-allowed' : 'bg-dark-800 border-dark-700 text-white'}`}
                    disabled={!isOwner}
                  >
                    <option value="LayoutDashboard">Dashboard</option>
                    <option value="Users">Users</option>
                    <option value="CreditCard">Credit Card</option>
                    <option value="Settings">Settings</option>
                    <option value="FileText">Document</option>
                    <option value="Image">Image</option>
                    <option value="Globe">Globe</option>
                    <option value="Shield">Shield</option>
                    <option value="Zap">Zap</option>
                    <option value="Star">Star</option>
                    <option value="Heart">Heart</option>
                    <option value="BookOpen">Book</option>
                    <option value="MessageCircle">Chat</option>
                    <option value="HelpCircle">Help</option>
                  </select>
                  {!item.isBuiltIn && (
                    <input
                      value={item.target}
                      onChange={e => updateNavItem(i, 'target', e.target.value)}
                      className={`flex-1 px-2 py-1 border rounded text-sm font-mono ${!isOwner ? 'bg-dark-800/50 border-dark-700/50 text-dark-400 cursor-not-allowed' : 'bg-dark-800 border-dark-700 text-white'}`}
                      disabled={!isOwner}
                      placeholder="/p/my-page"
                    />
                  )}
                  {item.isBuiltIn && (
                    <span className="flex-1 text-xs text-dark-500 font-mono">{item.target}</span>
                  )}
                  {isOwner ? (
                    <button
                      onClick={() => updateNavItem(i, 'visible', !item.visible)}
                      className={`p-1 rounded ${item.visible ? 'text-accent-emerald' : 'text-dark-600'}`}
                      title={item.visible ? 'Visible' : 'Hidden'}
                    >
                      {item.visible ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
                    </button>
                  ) : (
                    <span className={`p-1 ${item.visible ? 'text-accent-emerald' : 'text-dark-600'}`}>
                      {item.visible ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
                    </span>
                  )}
                  {isOwner && !item.isBuiltIn && (
                    <button onClick={() => removeNavItem(i)} className="p-1 text-red-400 hover:text-red-300">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              ))}
            </div>
            {isOwner && (
              <button onClick={addNavItem} className="flex items-center gap-2 mt-3 text-sm text-primary-400 hover:text-primary-300 transition-colors">
                <Plus className="w-4 h-4" /> Add Custom Nav Item
              </button>
            )}
          </Section>
        </div>
      )}

      {/* Pages Tab */}
      {tab === 'pages' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <p className="text-sm text-dark-400">Custom pages are served at /p/slug and can be linked from navigation.</p>
            {isOwner && (
              <button
                onClick={() => setEditingPage({ title: '', slug: '', htmlBody: '', isPublished: false, sortOrder: pages.length })}
                className="flex items-center gap-2 px-3 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors text-sm"
              >
                <Plus className="w-4 h-4" /> New Page
              </button>
            )}
          </div>

          {pagesLoading ? <LoadingSpinner size="lg" className="py-12" /> : (
            <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
              {pages.length === 0 ? (
                <div className="py-12 text-center text-dark-400">No custom pages yet.</div>
              ) : (
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-dark-800">
                      <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Title</th>
                      <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Slug</th>
                      <th className="text-left px-6 py-3 text-sm font-medium text-dark-400">Status</th>
                      {isOwner && <th className="text-right px-6 py-3 text-sm font-medium text-dark-400">Actions</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {pages.map(page => (
                      <tr key={page.id} className="border-b border-dark-800/50">
                        <td className="px-6 py-3 text-sm text-white font-medium">{page.title}</td>
                        <td className="px-6 py-3 text-sm text-dark-400 font-mono">/p/{page.slug}</td>
                        <td className="px-6 py-3">
                          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                            page.isPublished ? 'bg-accent-emerald/10 text-accent-emerald' : 'bg-dark-700 text-dark-400'
                          }`}>
                            {page.isPublished ? 'Published' : 'Draft'}
                          </span>
                        </td>
                        {isOwner && (
                          <td className="px-6 py-3 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <button onClick={() => setEditingPage(page)} className="text-xs px-3 py-1.5 rounded-lg border border-dark-700 text-dark-300 hover:text-white transition-colors">
                                Edit
                              </button>
                              <button onClick={() => handlePageDelete(page.id)} className="text-xs px-3 py-1.5 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors">
                                Delete
                              </button>
                            </div>
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* Page Editor Modal */}
          {isOwner && editingPage && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
              <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-2xl mx-4 w-full max-h-[90vh] overflow-y-auto">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-white">{editingPage.id ? 'Edit Page' : 'New Page'}</h3>
                  <button onClick={() => setEditingPage(null)} className="text-dark-400 hover:text-white"><X className="w-5 h-5" /></button>
                </div>
                <div className="space-y-4">
                  <Field label="Title">
                    <input value={editingPage.title || ''} onChange={e => setEditingPage(p => p ? { ...p, title: e.target.value } : p)} className={inputClass} />
                  </Field>
                  <Field label="Slug" description="URL will be /p/your-slug">
                    <input value={editingPage.slug || ''} onChange={e => setEditingPage(p => p ? { ...p, slug: e.target.value } : p)} className={`${inputClass} font-mono`} placeholder="my-page" />
                  </Field>
                  <Field label="HTML Body">
                    <textarea
                      value={editingPage.htmlBody || ''}
                      onChange={e => setEditingPage(p => p ? { ...p, htmlBody: e.target.value } : p)}
                      className={`${inputClass} font-mono text-xs h-48`}
                    />
                  </Field>
                  <Field label="Meta Description (SEO)">
                    <input value={editingPage.metaDescription || ''} onChange={e => setEditingPage(p => p ? { ...p, metaDescription: e.target.value } : p)} className={inputClass} />
                  </Field>
                  <Field label="Published">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="checkbox" checked={editingPage.isPublished || false} onChange={e => setEditingPage(p => p ? { ...p, isPublished: e.target.checked } : p)} className="rounded" />
                      <span className="text-sm text-dark-300">Published and accessible at /p/{editingPage.slug || '...'}</span>
                    </label>
                  </Field>
                  <div className="flex gap-3 justify-end pt-2">
                    <button onClick={() => setEditingPage(null)} className="px-4 py-2 text-sm text-dark-300 hover:text-white transition-colors">Cancel</button>
                    <button onClick={handlePageSave} disabled={pageSaving} className="px-4 py-2 text-sm bg-primary-500 text-white rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors">
                      {pageSaving ? 'Saving...' : 'Save Page'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Media Tab */}
      {tab === 'media' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <p className="text-sm text-dark-400">Upload images and files to use in your landing page, custom pages, and dashboard content.</p>
            {isOwner && (
              <div className="flex items-center gap-2">
                <input ref={mediaInputRef} type="file" accept="image/*,.pdf,.svg" className="hidden" onChange={e => {
                  const file = e.target.files?.[0];
                  if (file) handleMediaUpload(file);
                  e.target.value = '';
                }} />
                <button
                  onClick={() => mediaInputRef.current?.click()}
                  disabled={uploading}
                  className="flex items-center gap-2 px-3 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 disabled:opacity-50 transition-colors text-sm"
                >
                  <Upload className="w-4 h-4" /> {uploading ? 'Uploading...' : 'Upload File'}
                </button>
              </div>
            )}
          </div>

          {mediaLoading ? <LoadingSpinner size="lg" className="py-12" /> : (
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
              {media.length === 0 ? (
                <div className="col-span-full py-12 text-center text-dark-400 bg-dark-900/50 border border-dark-800 rounded-2xl">
                  No media files yet. Upload images to use in your content.
                </div>
              ) : media.map(item => (
                <div key={item.id} className="bg-dark-900/50 border border-dark-800 rounded-xl overflow-hidden group">
                  {item.contentType.startsWith('image/') ? (
                    <div className="aspect-square bg-dark-800 flex items-center justify-center">
                      <img src={item.url} alt={item.filename} className="max-w-full max-h-full object-contain" />
                    </div>
                  ) : (
                    <div className="aspect-square bg-dark-800 flex items-center justify-center">
                      <FileText className="w-12 h-12 text-dark-600" />
                    </div>
                  )}
                  <div className="p-3">
                    <p className="text-xs text-white truncate font-medium">{item.filename}</p>
                    <p className="text-xs text-dark-500 mt-0.5">{(item.size / 1024).toFixed(1)} KB</p>
                    <div className="flex items-center gap-2 mt-2">
                      <button
                        onClick={() => navigator.clipboard.writeText(item.url)}
                        className="text-xs text-primary-400 hover:text-primary-300"
                      >
                        Copy URL
                      </button>
                      {isOwner && (
                        <button
                          onClick={() => handleMediaDelete(item.key)}
                          className="text-xs text-red-400 hover:text-red-300"
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// --- Sub-components ---

const inputClass = 'w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 transition-colors text-sm';
const disabledInputClass = 'w-full px-3 py-2 bg-dark-800/50 border border-dark-700/50 rounded-lg text-dark-400 placeholder-dark-500 cursor-not-allowed transition-colors text-sm';

function Section({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
      <h3 className="text-md font-semibold text-white mb-1">{title}</h3>
      {description && <p className="text-sm text-dark-400 mb-4">{description}</p>}
      {!description && <div className="mb-4" />}
      <div className="space-y-4">{children}</div>
    </div>
  );
}

function Field({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium text-dark-300 mb-1">{label}</label>
      {description && <p className="text-xs text-dark-500 mb-1.5">{description}</p>}
      {children}
    </div>
  );
}

function ColorField({ label, value, onChange, isOwner = true }: { label: string; value: string; onChange: (v: string) => void; isOwner?: boolean }) {
  return (
    <Field label={label}>
      <div className="flex items-center gap-2">
        <input
          type="color"
          value={value || '#0ea5e9'}
          onChange={e => onChange(e.target.value)}
          className={`w-10 h-10 rounded-lg border border-dark-700 bg-transparent ${isOwner ? 'cursor-pointer' : 'cursor-not-allowed opacity-50'}`}
          disabled={!isOwner}
        />
        <input
          value={value}
          onChange={e => onChange(e.target.value)}
          className={isOwner ? inputClass : disabledInputClass}
          disabled={!isOwner}
          placeholder="#0ea5e9"
        />
        {isOwner && value && (
          <button onClick={() => onChange('')} className="text-dark-500 hover:text-dark-300">
            <X className="w-4 h-4" />
          </button>
        )}
      </div>
    </Field>
  );
}

function AssetUploader({ label, currentUrl, onUpload, onDelete, isOwner = true }: {
  label: string;
  currentUrl: string;
  onUpload: (file: File) => void;
  onDelete: () => void;
  isOwner?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  return (
    <Field label={label}>
      <div className="flex items-center gap-3">
        {currentUrl ? (
          <div className="w-16 h-16 bg-dark-800 border border-dark-700 rounded-lg flex items-center justify-center overflow-hidden">
            <img src={currentUrl} alt={label} className="max-w-full max-h-full object-contain" />
          </div>
        ) : (
          <div className="w-16 h-16 bg-dark-800 border border-dark-700 border-dashed rounded-lg flex items-center justify-center">
            <Upload className="w-5 h-5 text-dark-600" />
          </div>
        )}
        {isOwner && (
          <div className="flex flex-col gap-1">
            <input ref={inputRef} type="file" accept="image/*" className="hidden" onChange={e => {
              const file = e.target.files?.[0];
              if (file) onUpload(file);
              e.target.value = '';
            }} />
            <button onClick={() => inputRef.current?.click()} className="text-xs text-primary-400 hover:text-primary-300">
              {currentUrl ? 'Replace' : 'Upload'}
            </button>
            {currentUrl && (
              <button onClick={onDelete} className="text-xs text-red-400 hover:text-red-300">Remove</button>
            )}
          </div>
        )}
      </div>
    </Field>
  );
}
