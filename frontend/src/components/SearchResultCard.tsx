import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import CardActions from "@mui/material/CardActions";
import Typography from "@mui/material/Typography";
import Chip from "@mui/material/Chip";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import type { SearchResult } from "../types";

function formatSize(bytes: number): string {
  if (!bytes) return "размер неизвестен";
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb >= 1) return `${gb.toFixed(2)} GB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toFixed(0)} MB`;
}

interface Props {
  result: SearchResult;
  onDownload: (result: SearchResult) => void;
  downloading: boolean;
}

export function SearchResultCard({ result, onDownload, downloading }: Props) {
  return (
    <Card variant="outlined">
      <CardContent>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
          <Chip label={result.source} size="small" />
          <Typography variant="body2" color="text.secondary">
            {formatSize(result.sizeBytes)} · сиды {result.seeders} · личи {result.leechers}
          </Typography>
        </Stack>
        <Typography variant="subtitle1">{result.title}</Typography>
      </CardContent>
      <CardActions>
        <Button size="small" variant="contained" disabled={downloading} onClick={() => onDownload(result)}>
          {downloading ? "Добавляем…" : "Скачать"}
        </Button>
      </CardActions>
    </Card>
  );
}
