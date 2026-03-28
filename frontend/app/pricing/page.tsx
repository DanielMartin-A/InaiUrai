'use client';

const PLANS = [
  { name: 'Free', price: '$0', sub: '3 tasks', features: ['Chief of Staff only', 'Solo tasks'] },
  { name: 'Solo', price: '$19', sub: '/month', features: ['All 16 roles', '50 tasks/mo', 'Telegram + PWA'] },
  { name: 'Team', price: '$99', sub: '/month', features: ['Multi-role projects', 'Up to 5 members', 'Priority support'], accent: true },
  { name: 'Department', price: '$999', sub: '/month', features: ['Proactive heartbeats', 'Up to 10 members', 'Slack'] },
  { name: 'Company', price: '$2,999', sub: '/month', features: ['Full AI workforce', 'REST API', 'Up to 50 members'] },
];

export default function PricingPage() {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="flex items-center justify-between px-5 py-3 border-b border-neutral-100">
        <a href="/" className="text-sm font-medium text-neutral-400 hover:text-neutral-600 transition">InaiUrai</a>
        <a href="/chat" className="text-xs text-neutral-400 hover:text-neutral-600 transition">&larr; Back</a>
      </header>

      <main className="flex-1 flex flex-col items-center justify-center px-6 py-16">
        <h1 className="text-xl font-medium text-neutral-800 mb-12">Pricing</h1>

        <div className="grid grid-cols-1 sm:grid-cols-5 gap-3 max-w-4xl w-full">
          {PLANS.map((plan) => (
            <div key={plan.name} className={`rounded-xl p-5 space-y-4 ${
              plan.accent ? 'border-2 border-neutral-900' : 'border border-neutral-200'
            }`}>
              <div>
                <p className="text-sm font-medium">{plan.name}</p>
                <p className="text-2xl font-medium mt-1">{plan.price}<span className="text-xs text-neutral-400 font-normal">{plan.sub}</span></p>
              </div>
              <div className="space-y-1.5">
                {plan.features.map((f) => (
                  <p key={f} className="text-xs text-neutral-500">{f}</p>
                ))}
              </div>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
