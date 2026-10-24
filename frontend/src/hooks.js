import { useEffect, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryString, request, requestPhoto } from "./lib/api";
import { queryKeys } from "./queryClient";

export function useMachines() {
  const query = useQuery({
    queryKey: queryKeys.machines,
    queryFn: async ({ signal }) => {
      const page = await request("/machines", { signal });
      return page.items;
    },
  });

  return {
    machines: query.data ?? [],
    loading: query.isLoading,
    error: query.error ?? null,
    retry: () => query.refetch(),
  };
}

export function useCatalog() {
  const machinesQuery = useQuery({
    queryKey: queryKeys.machines,
    queryFn: async ({ signal }) => {
      const page = await request("/machines", { signal });
      return page.items;
    },
  });

  const shiftsQuery = useQuery({
    queryKey: queryKeys.shifts,
    queryFn: async ({ signal }) => {
      const shifts = await request("/shifts", { signal });
      return shifts.items;
    },
  });

  return {
    machines: machinesQuery.data ?? [],
    shifts: shiftsQuery.data ?? [],
    loading: machinesQuery.isLoading || shiftsQuery.isLoading,
    error: machinesQuery.error || shiftsQuery.error || null,
    retry: () => {
      machinesQuery.refetch();
      shiftsQuery.refetch();
    },
  };
}

export function useBook(id) {
  const enabled = Boolean(id);
  const query = useQuery({
    queryKey: queryKeys.book(id),
    queryFn: ({ signal }) =>
      request(`/logbooks/${encodeURIComponent(id)}`, { signal }),
    enabled,
  });

  return {
    book: query.data ?? null,
    loading: enabled && query.isLoading,
    error: query.error ?? null,
    retry: () => query.refetch(),
  };
}

export function usePhoto(bookId, photoId) {
  const enabled = Boolean(bookId && photoId);
  const query = useQuery({
    queryKey: queryKeys.photo(bookId, photoId),
    queryFn: ({ signal }) =>
      request(
        `/logbooks/${encodeURIComponent(bookId)}/photos/${encodeURIComponent(photoId)}`,
        { signal, blob: true },
      ),
    enabled,
    staleTime: Infinity,
  });

  const url = useMemo(() => {
    if (!query.data) return "";
    return URL.createObjectURL(query.data);
  }, [query.data]);

  useEffect(() => {
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [url]);

  return {
    url,
    loading: enabled && query.isLoading,
    error: query.error ?? null,
    retry: () => query.refetch(),
  };
}

export function usePhotoUrl(url) {
  const enabled = Boolean(url);
  const query = useQuery({
    queryKey: queryKeys.photoUrl(url),
    queryFn: ({ signal }) => requestPhoto(url, signal),
    enabled,
    staleTime: Infinity,
  });

  const objectUrl = useMemo(() => {
    if (!query.data) return "";
    return URL.createObjectURL(query.data);
  }, [query.data]);

  useEffect(() => {
    return () => {
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [objectUrl]);

  return {
    url: objectUrl,
    loading: enabled && query.isLoading,
    error: query.error ?? null,
    retry: () => query.refetch(),
  };
}
export function usePhotoMarkers(photoUrl) {
  const enabled = Boolean(photoUrl?.startsWith("/api/v1/"));
  const endpoint = enabled ? `${photoUrl.slice("/api/v1".length)}/markers` : "";
  const client = useQueryClient();
  const queryKey = queryKeys.photoMarkers(photoUrl);
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => request(endpoint, { signal }),
    enabled,
  });
  const mutation = useMutation({
    mutationFn: (marker) => request(endpoint, { method: "POST", body: marker }),
    onSuccess: (marker) => client.setQueryData(queryKey, (data) => ({
      ...data, items: [...(data?.items ?? []), marker],
    })),
  });
  return {
    markers: query.data?.items ?? [],
    canEdit: Boolean(query.data?.can_edit),
    loading: enabled && query.isLoading,
    saving: mutation.isPending,
    error: query.error ?? mutation.error,
    retry: () => { mutation.reset(); query.refetch(); },
    add: mutation.mutate,
  };
}

export function useShiftArchive(machineId, date, shiftId, options = {}) {
  const enabled = Boolean(options.enabled ?? (machineId && date && shiftId));
  const query = useQuery({
    queryKey: queryKeys.shiftArchive(machineId, date, shiftId),
    queryFn: async ({ signal }) => {
      const result = await request(
        `/logbooks?${queryString({
          machine_id: machineId,
          date_from: date,
          date_to: date,
          shift_id: shiftId,
          limit: 1,
        })}`,
        { signal },
      );
      return result.items[0] || null;
    },
    enabled,
  });

  return {
    book: query.data ?? null,
    loading: enabled && query.isLoading,
    error: query.error ?? null,
    retry: () => query.refetch(),
  };
}
