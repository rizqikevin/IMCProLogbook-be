import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 1000 * 60,
      gcTime: 1000 * 60 * 5,
    },
  },
});

export const queryKeys = {
  machines: ["machines"],
  shifts: ["shifts"],
  book: (id) => ["logbooks", String(id)],
  shiftArchive: (machineId, date, shiftId) => [
    "logbooks",
    "shift",
    String(machineId),
    String(date),
    String(shiftId),
  ],
  photo: (bookId, photoId) => ["photos", String(bookId), String(photoId)],
  photoUrl: (url) => ["photos", "url", String(url)],
};
