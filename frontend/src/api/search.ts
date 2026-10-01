import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "./client";
import type { SearchResult } from "../types";

export function useSearch(query: string) {
  return useQuery({
    queryKey: ["search", query],
    queryFn: () => apiFetch<SearchResult[]>(`/search?query=${encodeURIComponent(query)}`),
    enabled: query.trim().length > 0,
  });
}
