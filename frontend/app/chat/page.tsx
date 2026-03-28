'use client';
import { useState, useEffect, useRef, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  roleSlug?: string;
  timestamp: Date;
  status?: 'sending' | 'done' | 'error';
}

function ChatContent() {
  const searchParams = useSearchParams();
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [connected, setConnected] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const initialSentRef = useRef(false);

  const getToken = () => {
    if (typeof window === 'undefined') return '';
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get('token');
    if (urlToken) {
      localStorage.setItem('inaiurai_token', urlToken);
      params.delete('token');
      const clean = params.toString();
      window.history.replaceState({}, '', clean ? `?${clean}` : window.location.pathname);
      return urlToken;
    }
    return localStorage.getItem('inaiurai_token') || '';
  };

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const WS_URL = (process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080/ws/chat');
    const connect = () => {
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onopen = () => {
        ws.send(JSON.stringify({ type: 'auth', token }));
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          switch (data.type) {
            case 'auth_ok':
              setConnected(true);
              break;
            case 'ack':
              break;
            case 'routing':
              updatePendingMessage(`Routed to ${data.role_slug}...`);
              break;
            case 'done':
              completePendingMessage(data.content, data.role_slug);
              break;
            case 'error':
              completePendingMessage(data.content || 'Something went wrong.', '');
              break;
            case 'pong':
              break;
          }
        } catch {}
      };

      ws.onclose = () => {
        setConnected(false);
        setTimeout(connect, 3000);
      };
      ws.onerror = () => ws.close();
    };

    connect();
    return () => { wsRef.current?.close(); };
  }, []);

  useEffect(() => {
    const q = searchParams.get('q');
    if (q && !initialSentRef.current) {
      initialSentRef.current = true;
      const timer = setTimeout(() => sendMessage(q), 500);
      return () => clearTimeout(timer);
    }
  }, [connected]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const updatePendingMessage = (statusText: string) => {
    setMessages(prev => prev.map(m =>
      m.status === 'sending' ? { ...m, content: statusText } : m
    ));
  };

  const completePendingMessage = (content: string, roleSlug: string) => {
    setMessages(prev => prev.map(m =>
      m.status === 'sending' ? { ...m, content, status: 'done', roleSlug: roleSlug || undefined } : m
    ));
  };

  const sendMessage = async (text: string) => {
    if (!text.trim()) return;
    const userMsg: Message = { id: Date.now().toString(), role: 'user', content: text.trim(), timestamp: new Date() };
    const assistantMsg: Message = { id: (Date.now() + 1).toString(), role: 'assistant', content: '', status: 'sending', timestamp: new Date() };
    setMessages(prev => [...prev, userMsg, assistantMsg]);
    setInput('');

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'message', content: text.trim() }));
      return;
    }

    try {
      const res = await fetch(`${API_URL}/api/v1/tasks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${getToken()}` },
        body: JSON.stringify({ input_text: text.trim() }),
      });
      const data = await res.json();
      completePendingMessage(data.output || data.error || 'No response.', data.role_slug || '');
    } catch {
      completePendingMessage('Something went wrong.', '');
    }
  };

  return (
    <div className="h-screen flex flex-col">
      <header className="flex items-center justify-between px-5 py-3 border-b border-neutral-100">
        <div className="flex items-center gap-2">
          <a href="/" className="text-sm font-medium text-neutral-400 hover:text-neutral-600 transition">InaiUrai</a>
          <div className={`w-1.5 h-1.5 rounded-full ${connected ? 'bg-emerald-400' : 'bg-neutral-300'}`} />
        </div>
        <button onClick={() => setSidebarOpen(!sidebarOpen)} className="text-xs text-neutral-400 hover:text-neutral-600 transition">
          {sidebarOpen ? 'Close' : 'History'}
        </button>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <main className="flex-1 overflow-y-auto">
          {messages.length === 0 ? (
            <div className="h-full flex items-center justify-center">
              <p className="text-neutral-300 text-lg">What do you need done?</p>
            </div>
          ) : (
            <div className="max-w-2xl mx-auto px-5 py-8 space-y-6">
              {messages.map((msg) => (
                <div key={msg.id}>
                  {msg.role === 'user' ? (
                    <div className="flex justify-end">
                      <div className="bg-neutral-100 rounded-2xl rounded-br-md px-4 py-3 max-w-lg">
                        <p className="text-sm">{msg.content}</p>
                      </div>
                    </div>
                  ) : (
                    <div>
                      {msg.roleSlug && (
                        <span className="text-[11px] text-neutral-400 mb-1 block">
                          {msg.roleSlug.split('-').map(w => w[0].toUpperCase() + w.slice(1)).join(' ')}
                        </span>
                      )}
                      {msg.status === 'sending' ? (
                        <div className="flex items-center gap-2 py-1">
                          <div className="flex gap-1">
                            <span className="w-1.5 h-1.5 bg-neutral-300 rounded-full animate-bounce" style={{animationDelay:'0ms'}} />
                            <span className="w-1.5 h-1.5 bg-neutral-300 rounded-full animate-bounce" style={{animationDelay:'150ms'}} />
                            <span className="w-1.5 h-1.5 bg-neutral-300 rounded-full animate-bounce" style={{animationDelay:'300ms'}} />
                          </div>
                          {msg.content && <span className="text-xs text-neutral-400">{msg.content}</span>}
                        </div>
                      ) : (
                        <div className="text-sm leading-relaxed text-neutral-700 whitespace-pre-wrap">{msg.content}</div>
                      )}
                    </div>
                  )}
                </div>
              ))}
              <div ref={endRef} />
            </div>
          )}
        </main>

        {sidebarOpen && (
          <aside className="w-72 border-l border-neutral-100 overflow-y-auto bg-white p-4">
            <p className="text-xs font-medium text-neutral-400 uppercase tracking-wider mb-3">Recent</p>
            <p className="text-xs text-neutral-300">No engagements yet.</p>
          </aside>
        )}
      </div>

      <div className="border-t border-neutral-100 px-5 py-3">
        <div className="max-w-2xl mx-auto flex items-end gap-2">
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Follow up..."
            rows={1}
            className="flex-1 bg-transparent text-sm text-neutral-800 placeholder:text-neutral-300 focus:outline-none resize-none py-2"
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(input); }
            }}
            onInput={(e) => {
              const t = e.target as HTMLTextAreaElement;
              t.style.height = 'auto';
              t.style.height = Math.min(t.scrollHeight, 120) + 'px';
            }}
          />
          <button
            onClick={() => sendMessage(input)}
            disabled={!input.trim()}
            className="text-sm text-neutral-400 disabled:opacity-20 hover:text-neutral-800 transition pb-2"
          >
            &uarr;
          </button>
        </div>
      </div>
    </div>
  );
}

export default function ChatPage() {
  return <Suspense><ChatContent /></Suspense>;
}
