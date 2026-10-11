import { createContext, useContext, useEffect, useRef, useState } from "react";
import { useAuth } from "../auth";

const DraftContext = createContext(null);
const emptyDraft = { text: "", photos: [], sending: false, error: null, sent: null };

export function MessageDrafts({ children }) {
  const { user } = useAuth();
  return <DraftStore key={user.id}>{children}</DraftStore>;
}

function DraftStore({ children }) {
  const [drafts, setDrafts] = useState({});
  const current = useRef(drafts);
  current.current = drafts;
  const urls = useRef(new Set());
  useEffect(() => {
    const warn = (event) => {
      if (Object.values(current.current).some((draft) => draft.text || draft.photos.length || draft.sending)) {
        event.preventDefault();
        event.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => {
      window.removeEventListener("beforeunload", warn);
      for (const url of urls.current) URL.revokeObjectURL(url);
    };
  }, []);
  function update(machineId, change) {
    setDrafts((old) => ({
      ...old,
      [machineId]: { ...(old[machineId] || emptyDraft), ...change },
    }));
  }
  function preview(file) {
    const url = URL.createObjectURL(file);
    urls.current.add(url);
    return { file, url };
  }
  function release(photo) {
    URL.revokeObjectURL(photo.url);
    urls.current.delete(photo.url);
  }
  return <DraftContext.Provider value={{ drafts, update, preview, release }}>{children}</DraftContext.Provider>;
}

export function useMessageDraft(machineId) {
  const store = useContext(DraftContext);
  return { ...store, draft: store.drafts[machineId] || emptyDraft };
}
