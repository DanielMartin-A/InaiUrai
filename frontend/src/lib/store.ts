import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Role = 'requester' | 'provider' | 'admin';

interface AuthState {
  token: string | null;
  role: Role | null;
  login: (token: string, role: Role) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      role: null,
      login: (token, role) => set({ token, role }),
      logout: () => set({ token: null, role: null }),
    }),
    { name: 'inaiurai-auth' }
  )
);
