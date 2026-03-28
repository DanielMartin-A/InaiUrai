'use client';
import { useState } from 'react';
import { useRouter } from 'next/navigation';

const EXAMPLES = [
  "Research top CRM tools for small agencies",
  "Draft a cold email to a Series A investor",
  "Analyze our competitor's pricing strategy",
  "Write a 90-day go-to-market plan",
];

export default function Home() {
  const [input, setInput] = useState('');
  const [focused, setFocused] = useState(false);
  const router = useRouter();

  const submit = () => {
    if (!input.trim()) return;
    router.push(`/chat?q=${encodeURIComponent(input.trim())}`);
  };

  return (
    <div className="h-screen flex flex-col items-center justify-center px-6">
      <div className="fixed top-6 left-6">
        <span className="text-sm font-medium tracking-tight text-neutral-400">InaiUrai</span>
      </div>

      <div className="w-full max-w-2xl space-y-6">
        <h1 className="text-center text-2xl font-medium text-neutral-800 tracking-tight">
          What do you need done?
        </h1>

        <div className={`relative rounded-2xl border transition-all duration-200 ${
          focused ? 'border-neutral-400 shadow-sm' : 'border-neutral-200'
        }`}>
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            placeholder="Describe the outcome you want..."
            rows={3}
            className="w-full bg-transparent rounded-2xl px-5 py-4 text-base text-neutral-800 placeholder:text-neutral-400 focus:outline-none resize-none"
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); }
            }}
          />
          <div className="flex items-center justify-between px-4 pb-3">
            <div className="flex gap-2 overflow-x-auto">
              {EXAMPLES.map((ex, i) => (
                <button
                  key={i}
                  onClick={() => setInput(ex)}
                  className="shrink-0 text-xs text-neutral-400 border border-neutral-200 px-3 py-1 rounded-full hover:text-neutral-600 hover:border-neutral-300 transition"
                >
                  {ex}
                </button>
              ))}
            </div>
            <button
              onClick={submit}
              disabled={!input.trim()}
              className="shrink-0 ml-3 bg-neutral-900 text-white text-sm px-4 py-1.5 rounded-full disabled:opacity-20 hover:bg-black transition"
            >
              &rarr;
            </button>
          </div>
        </div>
      </div>

      <div className="fixed bottom-6 text-xs text-neutral-300">
        16 AI executives &middot; 5 divisions
      </div>
    </div>
  );
}
