const API_URL = process.env.NEXT_PUBLIC_API_URL || "";

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
}

export async function apiFetch<T>(
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") : null;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_URL}${path}`, {
    method: options.method || "GET",
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("token");
      localStorage.removeItem("refreshToken");
      window.location.href = "/login";
    }
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(error.message || res.statusText);
  }

  if (res.status === 204) return {} as T;
  return res.json();
}

export function useAuth() {
  const login = async (email: string, password: string) => {
    const data = await apiFetch<{
      token: string;
      refreshToken: string;
      userId: string;
      orgId: string;
    }>("/api/v1/auth/login", {
      method: "POST",
      body: { email, password },
    });
    localStorage.setItem("token", data.token);
    localStorage.setItem("refreshToken", data.refreshToken);
    localStorage.setItem("orgId", data.orgId);
    return data;
  };

  const register = async (
    orgName: string,
    email: string,
    password: string,
    name: string
  ) => {
    const data = await apiFetch<{
      token: string;
      refreshToken: string;
      userId: string;
      orgId: string;
    }>("/api/v1/auth/register", {
      method: "POST",
      body: { orgName, email, password, name },
    });
    localStorage.setItem("token", data.token);
    localStorage.setItem("refreshToken", data.refreshToken);
    localStorage.setItem("orgId", data.orgId);
    return data;
  };

  const logout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("refreshToken");
    localStorage.removeItem("orgId");
    window.location.href = "/login";
  };

  const isAuthenticated = () => {
    return typeof window !== "undefined" && !!localStorage.getItem("token");
  };

  return { login, register, logout, isAuthenticated };
}
