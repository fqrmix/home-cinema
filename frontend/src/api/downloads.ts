import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";
import type { Download, SearchResult } from "../types";

export function useDownloads() {
  return useQuery({
    queryKey: ["downloads"],
    queryFn: () => apiFetch<Download[]>("/downloads"),
    // Polling instead of a websocket: single-user personal tool, a 5s
    // delay on progress updates is an acceptable trade for not having to
    // run a push channel.
    refetchInterval: 5000,
  });
}

export function useCreateDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (result: SearchResult) =>
      apiFetch<Download>("/downloads", {
        method: "POST",
        body: JSON.stringify(result),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["downloads"] }),
  });
}

export function useDeleteDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiFetch<void>(`/downloads/${id}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["downloads"] }),
  });
}
