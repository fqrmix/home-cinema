import { useMutation } from "@tanstack/react-query";
import { apiFetch, setToken } from "./client";

interface LoginResponse {
  token: string;
  expiresAt: string;
}

export function useLogin() {
  return useMutation({
    mutationFn: (password: string) =>
      apiFetch<LoginResponse>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      }),
    onSuccess: (data) => setToken(data.token),
  });
}
