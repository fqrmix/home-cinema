import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";
import { useDeleteDownload, useDownloads } from "../api/downloads";
import { DownloadListItem } from "../components/DownloadListItem";

export function LibraryPage() {
  const { data: downloads, isLoading } = useDownloads();
  const deleteDownload = useDeleteDownload();

  if (isLoading) {
    return <CircularProgress />;
  }

  if (!downloads || downloads.length === 0) {
    return <Typography color="text.secondary">Пока ничего не скачано</Typography>;
  }

  return (
    <Box display="grid" gridTemplateColumns="repeat(auto-fill, minmax(180px, 1fr))" gap={2}>
      {downloads.map((download) => (
        <DownloadListItem key={download.id} download={download} onDelete={(id) => deleteDownload.mutate(id)} />
      ))}
    </Box>
  );
}
