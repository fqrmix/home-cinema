export type Source = "rutracker" | "rutor";
export type Status = "queued" | "downloading" | "seeding" | "completed" | "error";

export interface SearchResult {
  source: Source;
  title: string;
  sizeBytes: number;
  seeders: number;
  leechers: number;
  magnetOrTorrentUrl: string;
  publishDate: string;
}

export interface MovieMetadata {
  downloadId: string;
  tmdbId: number;
  title: string;
  originalTitle: string;
  overview: string;
  posterPath: string;
  releaseYear: number;
  voteAverage: number;
}

export interface Download {
  id: string;
  source: Source;
  sourceTitle: string;
  magnetOrTorrentUrl: string;
  transmissionHash?: string;
  status: Status;
  progressPercent: number;
  downloadDir?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
  metadata?: MovieMetadata;
}
