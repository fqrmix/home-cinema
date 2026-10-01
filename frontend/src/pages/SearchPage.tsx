import { useState, type FormEvent } from "react";
import Box from "@mui/material/Box";
import TextField from "@mui/material/TextField";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";
import { useSearch } from "../api/search";
import { useCreateDownload } from "../api/downloads";
import { SearchResultCard } from "../components/SearchResultCard";
import type { SearchResult } from "../types";

export function SearchPage() {
  const [input, setInput] = useState("");
  const [query, setQuery] = useState("");
  const { data: results, isFetching } = useSearch(query);
  const createDownload = useCreateDownload();

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    setQuery(input.trim());
  };

  return (
    <Box>
      <Box component="form" onSubmit={handleSubmit} sx={{ display: "flex", gap: 2, mb: 3 }}>
        <TextField
          fullWidth
          label="Название фильма"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          autoFocus
        />
      </Box>

      {isFetching ? <CircularProgress /> : null}

      {!isFetching && query && (results?.length ?? 0) === 0 ? (
        <Typography color="text.secondary">Ничего не найдено</Typography>
      ) : null}

      <Box display="grid" gridTemplateColumns="repeat(auto-fill, minmax(320px, 1fr))" gap={2}>
        {results?.map((result) => (
          <SearchResultCard
            key={`${result.source}-${result.magnetOrTorrentUrl}`}
            result={result}
            onDownload={(chosen: SearchResult) => createDownload.mutate(chosen)}
            downloading={
              createDownload.isPending &&
              createDownload.variables?.magnetOrTorrentUrl === result.magnetOrTorrentUrl
            }
          />
        ))}
      </Box>
    </Box>
  );
}
