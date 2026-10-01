import Card from "@mui/material/Card";
import CardMedia from "@mui/material/CardMedia";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";
import LinearProgress from "@mui/material/LinearProgress";
import Stack from "@mui/material/Stack";
import IconButton from "@mui/material/IconButton";
import DeleteIcon from "@mui/icons-material/Delete";
import { StatusChip } from "./StatusChip";
import type { Download } from "../types";

const TMDB_POSTER_BASE = "https://image.tmdb.org/t/p/w342";

interface Props {
  download: Download;
  onDelete: (id: string) => void;
}

export function DownloadListItem({ download, onDelete }: Props) {
  const meta = download.metadata;
  const poster = meta?.posterPath ? `${TMDB_POSTER_BASE}${meta.posterPath}` : undefined;
  const displayTitle = meta?.title || download.sourceTitle;

  return (
    <Card variant="outlined" sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
      {poster ? (
        <CardMedia
          component="img"
          image={poster}
          alt={displayTitle}
          sx={{ aspectRatio: "2 / 3", objectFit: "cover" }}
        />
      ) : (
        <Stack
          sx={{ aspectRatio: "2 / 3", bgcolor: "action.hover" }}
          alignItems="center"
          justifyContent="center"
        >
          <Typography variant="body2" color="text.secondary" sx={{ p: 2, textAlign: "center" }}>
            {displayTitle}
          </Typography>
        </Stack>
      )}
      <CardContent sx={{ flexGrow: 1 }}>
        <Typography variant="subtitle2" noWrap title={displayTitle}>
          {displayTitle}
        </Typography>
        {meta?.releaseYear ? (
          <Typography variant="body2" color="text.secondary">
            {meta.releaseYear} · ★ {meta.voteAverage.toFixed(1)}
          </Typography>
        ) : null}
        <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1 }}>
          <StatusChip status={download.status} />
          <IconButton size="small" sx={{ ml: "auto" }} onClick={() => onDelete(download.id)} aria-label="удалить">
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Stack>
        {download.status === "downloading" || download.status === "queued" ? (
          <LinearProgress variant="determinate" value={download.progressPercent} sx={{ mt: 1 }} />
        ) : null}
        {download.status === "error" && download.errorMessage ? (
          <Typography variant="caption" color="error">
            {download.errorMessage}
          </Typography>
        ) : null}
      </CardContent>
    </Card>
  );
}
