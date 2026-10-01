import Chip from "@mui/material/Chip";
import type { Status } from "../types";

const labels: Record<Status, string> = {
  queued: "В очереди",
  downloading: "Скачивается",
  seeding: "Раздаётся",
  completed: "Готово",
  error: "Ошибка",
};

const colors: Record<Status, "default" | "primary" | "success" | "error"> = {
  queued: "default",
  downloading: "primary",
  seeding: "primary",
  completed: "success",
  error: "error",
};

export function StatusChip({ status }: { status: Status }) {
  return <Chip label={labels[status]} color={colors[status]} size="small" />;
}
