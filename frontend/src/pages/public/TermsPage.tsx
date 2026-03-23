import { Link } from 'react-router-dom';

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-700">
      {/* Nav */}
      <header className="border-b border-slate-200 px-6 py-4 bg-white/80 backdrop-blur-lg">
        <div className="max-w-3xl mx-auto flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center">
              <span className="text-white font-bold text-sm">ML</span>
            </div>
            <span className="font-semibold text-slate-900 text-lg">MCPLens</span>
          </Link>
          <Link to="/" className="text-sm text-slate-500 hover:text-slate-900 transition-colors">
            Back to Home
          </Link>
        </div>
      </header>

      <main className="max-w-3xl mx-auto px-4 py-12">
        <div className="bg-white border border-slate-200 rounded-2xl shadow-sm p-8 sm:p-12">
          <h1 className="text-3xl font-bold text-slate-900 mb-2">Terms of Service</h1>
          <p className="text-sm text-slate-400 mb-8">Last updated: March 23, 2026</p>

          <div className="prose prose-slate max-w-none space-y-6 text-slate-600 leading-relaxed">
            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">1. Service Description</h2>
              <p>
                MCPLens ("the Service") scans publicly accessible MCP (Model Context Protocol)
                endpoints associated with Shopify stores and other e-commerce platforms. The
                Service generates agent readiness scores, identifies issues, and provides fix
                recommendations to help store owners optimize their stores for AI buyer agents.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">2. Acceptable Use</h2>
              <p>You agree to use the Service only for lawful purposes. You may not:</p>
              <ul className="list-disc pl-6 space-y-1 mt-2">
                <li>Use the Service to harass, abuse, or harm others or their systems.</li>
                <li>Perform excessive automated scanning beyond reasonable use (e.g., continuously scanning the same endpoint in a loop).</li>
                <li>Attempt to reverse-engineer, decompile, or derive source code from the Service.</li>
                <li>Resell scan results without an Agency plan or explicit written permission.</li>
                <li>Use the Service to exploit vulnerabilities discovered during scanning.</li>
              </ul>
              <p className="mt-2">
                We reserve the right to throttle, suspend, or terminate accounts that violate
                these terms or engage in abusive behavior.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">3. Account Terms</h2>
              <p>
                A valid email address is required to create an account and access paid features.
                You are responsible for maintaining the security of your account and password.
                MCPLens will not be liable for any loss or damage resulting from unauthorized
                access to your account.
              </p>
              <p className="mt-2">
                Free-tier scanning is available without an account. An email address is required
                to unlock fix instructions for individual scan results.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">4. Billing and Payments</h2>
              <p>
                Paid plans are billed monthly via Stripe. You may cancel your subscription at
                any time from your account settings. Cancellation takes effect at the end of the
                current billing period. No refunds are provided for partial months.
              </p>
              <p className="mt-2">
                We reserve the right to change pricing with 30 days' notice. Existing
                subscribers will be notified by email before any price changes take effect.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">5. Data and Scanning</h2>
              <p>
                MCPLens scans only publicly accessible MCP endpoints. We do not access private
                APIs, store credentials, admin panels, customer data, or any non-public
                information. Scans interact with the same public interface that AI buyer agents
                use.
              </p>
              <p className="mt-2">
                You represent that you have the authority to scan the domains you submit. While
                scanning public endpoints is generally permissible, you are responsible for
                ensuring compliance with any applicable terms of the endpoints you scan.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">6. Scores Disclaimer</h2>
              <p>
                Agent readiness scores are automated estimates generated by algorithmic
                analysis of public endpoint behavior. Scores are provided for informational
                purposes only and <strong>do not guarantee commercial outcomes</strong>,
                including but not limited to increased sales, improved conversion rates, or
                higher visibility to AI buyer agents.
              </p>
              <p className="mt-2">
                Scores may vary between scans due to changes in endpoint behavior, network
                conditions, or updates to our scoring methodology.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">7. Aggregate Data</h2>
              <p>
                We may use anonymized, aggregated scan data to produce benchmarks, industry
                reports, and statistical analyses. No individually identifiable information
                will be included in aggregate data publications.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">8. Limitation of Liability</h2>
              <p>
                The Service is provided "as is" without warranties of any kind, express or
                implied. MCPLens shall not be liable for any indirect, incidental, special,
                consequential, or punitive damages arising from your use of the Service,
                including but not limited to loss of profits, data, or business opportunities.
              </p>
              <p className="mt-2">
                Our total liability for any claim arising from your use of the Service shall
                not exceed the amount you paid to MCPLens in the twelve months preceding the
                claim.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">9. Changes to Terms</h2>
              <p>
                We may update these Terms of Service from time to time. Material changes will
                be communicated via email or a prominent notice on the Service. Your continued
                use of the Service after changes take effect constitutes acceptance of the
                revised terms.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-semibold text-slate-900 mt-8 mb-3">10. Contact</h2>
              <p>
                If you have questions about these terms, please contact us at{' '}
                <a href="mailto:hello@transparentfunnel.com" className="text-blue-600 hover:text-blue-700 underline">
                  hello@transparentfunnel.com
                </a>.
              </p>
            </section>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-200 px-6 py-6 mt-12 bg-white">
        <div className="max-w-3xl mx-auto flex flex-wrap items-center justify-center gap-5 text-sm text-slate-400">
          <span>&copy; {new Date().getFullYear()} MCPLens</span>
          <Link to="/terms" className="hover:text-slate-900 transition-colors">Terms</Link>
          <Link to="/privacy" className="hover:text-slate-900 transition-colors">Privacy</Link>
        </div>
      </footer>
    </div>
  );
}
