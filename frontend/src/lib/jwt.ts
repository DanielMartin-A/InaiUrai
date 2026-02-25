/** Decode JWT payload without verification (client-side). API validates the token. */
export function decodeJwtPayload(token: string): { sub?: string; role?: string } {
  try {
    const base64 = token.split('.')[1];
    if (!base64) return {};
    const json = atob(base64.replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(json) as { sub?: string; role?: string };
  } catch {
    return {};
  }
}
