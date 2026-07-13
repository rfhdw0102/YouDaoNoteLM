import axios, { type InternalAxiosRequestConfig, type AxiosResponse } from 'axios';

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 60000,
  headers: { 'Content-Type': 'application/json' },
});

// Flag to prevent concurrent refresh attempts
let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

function subscribeTokenRefresh(cb: (token: string) => void) {
  refreshSubscribers.push(cb);
}

function onTokenRefreshed(newToken: string) {
  refreshSubscribers.forEach((cb) => cb(newToken));
  refreshSubscribers = [];
}

function clearAuth(reason?: string) {
  sessionStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  localStorage.removeItem('user');
  window.location.href = reason ? `/login?reason=${reason}` : '/login';
}

// Request: attach access_token
client.interceptors.request.use((config) => {
  const token = sessionStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Check if response indicates token issues
function isTokenError(data: any): boolean {
  return data && (data.code === 1005 || data.code === 1006);
}

// Check if response indicates the user has been disabled (1004)
function isUserDisabled(data: any): boolean {
  return data && data.code === 1004;
}

export async function doRefreshToken(): Promise<string | null> {
  const refreshToken = localStorage.getItem('refresh_token');
  if (!refreshToken) return null;
  try {
    const res = await axios.post('/api/v1/auth/refresh', { refresh_token: refreshToken });
    if (res.data.code === 0) {
      const { access_token, refresh_token: newRefresh } = res.data.data;
      sessionStorage.setItem('access_token', access_token);
      localStorage.setItem('refresh_token', newRefresh);
      return access_token;
    }
    return null;
  } catch {
    return null;
  }
}

// Response interceptor
client.interceptors.response.use(
  (response: AxiosResponse) => {
    const data = response.data;

    // 用户被禁用 → 立即强制退出,不尝试刷新 token
    if (isUserDisabled(data)) {
      clearAuth('disabled');
      return Promise.reject(new Error('user_disabled'));
    }

    // Backend returns HTTP 200 but code 1005/1006 → token issue
    if (isTokenError(data)) {
      const originalRequest = response.config as InternalAxiosRequestConfig & { _retry?: boolean };

      // Already retried this request, don't loop
      if (originalRequest._retry) {
        clearAuth();
        return Promise.reject(new Error('token_refresh_failed'));
      }

      // If already refreshing, queue this request
      if (isRefreshing) {
        return new Promise((resolve) => {
          subscribeTokenRefresh((newToken) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`;
            originalRequest._retry = true;
            resolve(client(originalRequest));
          });
        });
      }

      // Start refresh
      isRefreshing = true;
      originalRequest._retry = true;

      return doRefreshToken().then((newToken) => {
        isRefreshing = false;
        if (newToken) {
          onTokenRefreshed(newToken);
          originalRequest.headers.Authorization = `Bearer ${newToken}`;
          return client(originalRequest);
        } else {
          // Refresh failed
          refreshSubscribers = [];
          clearAuth();
          return Promise.reject(new Error('token_refresh_failed'));
        }
      });
    }

    return response;
  },
  async (error) => {
    // HTTP 4xx/5xx errors - check if it's a token issue in the response body
    const data = error.response?.data;
    // 用户被禁用 → 立即强制退出
    if (isUserDisabled(data)) {
      clearAuth('disabled');
      return Promise.reject(error);
    }
    if (isTokenError(data)) {
      const originalRequest = error.config;
      if (originalRequest._retry) {
        clearAuth();
        return Promise.reject(error);
      }

      if (isRefreshing) {
        return new Promise((resolve) => {
          subscribeTokenRefresh((newToken) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`;
            originalRequest._retry = true;
            resolve(client(originalRequest));
          });
        });
      }

      isRefreshing = true;
      originalRequest._retry = true;

      const newToken = await doRefreshToken();
      isRefreshing = false;
      if (newToken) {
        onTokenRefreshed(newToken);
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return client(originalRequest);
      } else {
        refreshSubscribers = [];
        clearAuth();
        return Promise.reject(error);
      }
    }

    return Promise.reject(error);
  }
);

export default client;
