import { create } from 'zustand';

interface AuthState {
  adminToken: string | null;
  userToken: string | null;
  isAuthenticated: boolean;
  userInfo: {
    id: string;
    username: string;
    email: string;
    role: string;
  } | null;
  setAdminToken: (token: string | null) => void;
  setUserToken: (token: string | null) => void;
  setAuthenticated: (auth: boolean) => void;
  setUserInfo: (info: {
    id: string;
    username: string;
    email: string;
    role: string;
  } | null) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  adminToken: null,
  userToken: null,
  isAuthenticated: false,
  userInfo: null,
  setAdminToken: (token) => set({ adminToken: token }),
  setUserToken: (token) => set({ userToken: token }),
  setAuthenticated: (auth) => set({ isAuthenticated: auth }),
  setUserInfo: (info) => set({ userInfo: info }),
  logout: () => set({
    adminToken: null,
    userToken: null,
    isAuthenticated: false,
    userInfo: null,
  }),
}));