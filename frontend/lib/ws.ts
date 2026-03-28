export class InaiUraiWS {
  private ws: WebSocket | null = null;
  private token: string;
  private url: string;
  private onMessage: (data: any) => void;
  private onStatusChange: (connected: boolean) => void;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(url: string, token: string, onMessage: (data: any) => void, onStatusChange: (connected: boolean) => void) {
    this.url = url; this.token = token; this.onMessage = onMessage; this.onStatusChange = onStatusChange;
  }

  connect() {
    this.ws = new WebSocket(this.url);
    this.ws.onopen = () => { this.ws?.send(JSON.stringify({ type: 'auth', token: this.token })); this.onStatusChange(true); };
    this.ws.onmessage = (event) => { try { this.onMessage(JSON.parse(event.data)); } catch {} };
    this.ws.onclose = () => { this.onStatusChange(false); this.reconnectTimer = setTimeout(() => this.connect(), 3000); };
    this.ws.onerror = () => { this.ws?.close(); };
  }

  send(content: string) { if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ type: 'message', content })); }
  disconnect() { if (this.reconnectTimer) clearTimeout(this.reconnectTimer); this.ws?.close(); }
}
