import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { useAuth } from "../auth";
import { useMachines, usePhotoUrl } from "../hooks";
import { request, queryString } from "../lib/api";
import { formatDate, formatTime, validateSelection } from "../lib/format";
import { flattenPhotos, mergeMessages, messageForm, textLimit, validateMessage } from "../lib/messages";
import { Dialog, ErrorNotice, Icon, Loading } from "../components/common";
import { useMessageDraft } from "../components/MessageDrafts";
import MachinePhoto from "../components/MachinePhoto";
import MessagePhoto from "../components/MessagePhoto";
import PhotoDialog from "../components/PhotoDialog";
import Camera from "../components/Camera";

const dayFormat = new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "long", year: "numeric" });
const clockFormat = new Intl.DateTimeFormat("id-ID", { hour: "2-digit", minute: "2-digit" });

export default function Conversation() {
  const { machineId } = useParams();
  const catalog = useMachines();
  const machine = catalog.machines.find((item) => String(item.id) === machineId);
  if (catalog.loading) return <div className="page"><Loading>Memuat mesin…</Loading></div>;
  if (catalog.error) return <div className="page"><ErrorNotice error={catalog.error} onRetry={catalog.retry} /></div>;
  if (!machine) return <div className="page"><h1>Mesin tidak ditemukan</h1><Link className="button button-outline" to="/">Pilih mesin</Link></div>;
  return <MachineConversation key={machineId} machine={machine} />;
}


function MachineConversation({ machine }) {
  const machineId = String(machine.id);
  const { user } = useAuth();
  const { draft, update, preview, release } = useMessageDraft(machineId);
  const [searchParams, setSearchParams] = useSearchParams();
  const query = (searchParams.get("q") || "").trim();
  const [searchOpen, setSearchOpen] = useState(Boolean(query));
  const [searchText, setSearchText] = useState(query);
  const [searchError, setSearchError] = useState("");
  const searchInput = useRef(null);
  const searchToggle = useRef(null);
  const activeQuery = useRef(query);
  activeQuery.current = query;
  const [state, setState] = useState({ items: [], cursor: "", loading: true, error: null });
  const [attempt, setAttempt] = useState(0);
  const [more, setMore] = useState({ loading: false, error: null });
  const [selectionError, setSelectionError] = useState("");
  const [cameraOpen, setCameraOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState(null);
  const [deleteStatus, setDeleteStatus] = useState("");
  const deletePending = useRef(false);
  const deleteTrigger = useRef(null);
  const restoreDeleteFocus = useRef(false);
  const mounted = useRef(false);
  const deleted = useRef(new Set());
  for (const id of draft.deletedMessageIds || []) deleted.current.add(id);
  const timeline = useRef(null);
  const shell = useRef(null);
  const textarea = useRef(null);
  const fileInput = useRef(null);
  const scroll = useRef(null);
  const paging = useRef(null);
  const posting = useRef(false);
  const seenSent = useRef(null);
  const endpoint = `/machines/${machineId}/messages`;

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  useEffect(() => {
    if (confirmDelete || !restoreDeleteFocus.current) return;
    restoreDeleteFocus.current = false;
    const target = deleteTrigger.current;
    if (target?.isConnected) target.focus({ preventScroll: true });
    else timeline.current?.focus({ preventScroll: true });
  }, [confirmDelete]);

  function closeDelete() {
    if (deletePending.current) return;
    restoreDeleteFocus.current = true;
    setConfirmDelete(null);
    setDeleteError(null);
  }

  async function deleteMessage() {
    if (!confirmDelete || deletePending.current) return;
    const id = String(confirmDelete.id);
    deletePending.current = true;
    setDeleting(true);
    setDeleteError(null);
    try {
      await request(`${endpoint}/${id}`, { method: "DELETE" });
      deleted.current.add(id);
      update(machineId, (current) => ({
        deletedMessageIds: [...(current.deletedMessageIds || []), id],
        sent: String(current.sent?.id) === id ? null : current.sent,
      }));
      if (!mounted.current) return;
      setState((old) => ({ ...old, items: old.items.filter((message) => String(message.id) !== id) }));
      restoreDeleteFocus.current = true;
      setConfirmDelete(null);
      setDeleteStatus("Pesan dihapus.");
    } catch (error) {
      if (mounted.current) setDeleteError(error);
    } finally {
      deletePending.current = false;
      if (mounted.current) setDeleting(false);
    }
  }

  useLayoutEffect(() => {
    const input = textarea.current;
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, 144)}px`;
  }, [draft.text]);

  useEffect(() => {
    const viewport = window.visualViewport;
    if (!viewport) return;
    const resize = () => {
      shell.current.style.height = `${viewport.height}px`;
      shell.current.style.transform = `translateY(${viewport.offsetTop}px)`;
    };
    resize();
    viewport.addEventListener("resize", resize);
    viewport.addEventListener("scroll", resize);
    return () => {
      viewport.removeEventListener("resize", resize);
      viewport.removeEventListener("scroll", resize);
    };
  }, []);

  useEffect(() => {
    setSearchText(query);
    setSearchError("");
    if (query) setSearchOpen(true);
  }, [query]);

  useEffect(() => {
    if (searchOpen) searchInput.current?.focus();
  }, [searchOpen]);

  function commitSearch(event) {
    event.preventDefault();
    const value = searchText.trim();
    if (Array.from(value).length > 200 || value.includes("\0")) {
      setSearchError("Pencarian maksimal 200 karakter dan tidak boleh berisi karakter NUL.");
      return;
    }
    setSearchError("");
    setSearchText(value);
    const params = new URLSearchParams(searchParams);
    if (value) params.set("q", value);
    else params.delete("q");
    setSearchParams(params);
  }

  function clearSearch(close = false) {
    setSearchText("");
    setSearchError("");
    const params = new URLSearchParams(searchParams);
    params.delete("q");
    setSearchParams(params);
    if (close) {
      setSearchOpen(false);
      searchToggle.current?.focus();
    } else searchInput.current?.focus();
  }

  useEffect(() => {
    const controller = new AbortController();
    paging.current?.abort();
    paging.current = null;
    scroll.current = null;
    setMore({ loading: false, error: null });
    setState({ items: [], cursor: "", loading: true, error: null });
    request(`${endpoint}?${queryString({ limit: 20, q: query })}`, { signal: controller.signal })
      .then((page) => {
        if (controller.signal.aborted || activeQuery.current !== query) return;
        scroll.current = { bottom: true };
        setState({ items: page.items.filter((message) => !deleted.current.has(String(message.id))), cursor: page.next_cursor || "", loading: false, error: null });
      })
      .catch((error) => {
        if (!controller.signal.aborted && activeQuery.current === query && error.name !== "AbortError") setState({ items: [], cursor: "", loading: false, error });
      });
    return () => { controller.abort(); paging.current?.abort(); };
  }, [endpoint, attempt, query]);

  useEffect(() => {
    if (!draft.sent || seenSent.current === draft.sent.id || deleted.current.has(String(draft.sent.id))) return;
    if (query) {
      seenSent.current = draft.sent.id;
      setAttempt((value) => value + 1);
      return;
    }
    if (state.loading || state.error) return;
    seenSent.current = draft.sent.id;
    scroll.current = { bottom: true };
    setState((old) => ({ ...old, items: mergeMessages(old.items, [draft.sent]) }));
  }, [draft.sent, state.loading, state.error, query]);

  useLayoutEffect(() => {
    if (!scroll.current || !timeline.current) return;
    if (scroll.current.bottom) timeline.current.scrollTop = timeline.current.scrollHeight;
    else timeline.current.scrollTop += timeline.current.scrollHeight - scroll.current.height;
    scroll.current = null;
  }, [state.items]);

  async function loadOlder() {
    if (paging.current || !state.cursor) return;
    const controller = new AbortController();
    paging.current = controller;
    setMore({ loading: true, error: null });
    try {
      const page = await request(`${endpoint}?${queryString({ limit: 20, cursor: state.cursor, q: query })}`, { signal: controller.signal });
      if (controller.signal.aborted || activeQuery.current !== query) return;
      scroll.current = { height: timeline.current.scrollHeight };
      setState((old) => ({ ...old, items: mergeMessages(page.items, old.items).filter((message) => !deleted.current.has(String(message.id))), cursor: page.next_cursor || "" }));
      setMore({ loading: false, error: null });
    } catch (error) {
      if (!controller.signal.aborted && activeQuery.current === query && error.name !== "AbortError") setMore({ loading: false, error });
    } finally { if (paging.current === controller) paging.current = null; }
  }
  function handleCameraCapture(incoming) {
    const error = validateSelection(draft.photos, incoming);
    setSelectionError(error);
    if (error) throw new Error(error);
    update(machineId, { photos: [...draft.photos, ...incoming.map(preview)], error: null });
  }
  function choosePhotos(event) {
    const incoming = Array.from(event.target.files || []);
    event.target.value = "";
    const error = validateSelection(draft.photos, incoming);
    setSelectionError(error);
    if (!error) update(machineId, { photos: [...draft.photos, ...incoming.map(preview)], error: null });
  }

  async function send(event) {
    event.preventDefault();
    if (posting.current || draft.sending) return;
    const error = validateMessage(draft.text, draft.photos);
    if (error) { update(machineId, { error }); return; }
    posting.current = true;
    update(machineId, { sending: true, error: null, sent: null });
    try {
      const message = await request(endpoint, { method: "POST", body: messageForm(draft.text, draft.photos) });
      draft.photos.forEach(release);
      update(machineId, { text: "", photos: [], sending: false, error: null, sent: message });
      setSelectionError("");
    } catch (error) {
      update(machineId, { sending: false, error });
    } finally { posting.current = false; }
  }

  const count = Array.from(draft.text).length;
  const items = state.items.filter((message) => !deleted.current.has(String(message.id)));
  const allPhotos = useMemo(() => flattenPhotos(items), [items]);
  const [activePhotoId, setActivePhotoId] = useState(null);
  const activeIndex = allPhotos.findIndex((p) => String(p.id) === String(activePhotoId));
  const activePhoto = activeIndex >= 0 ? allPhotos[activeIndex] : null;
  const viewerPhoto = usePhotoUrl(activePhoto?.url);
  return (
    <div className="conversation-page" ref={shell}>
      <header className="conversation-heading">
        <Link className="icon-button" to="/" aria-label="Pilih mesin lain" title="Pilih mesin lain"><Icon name="back" /></Link>
        <MachinePhoto name={machine.name} decorative />
        <div><h1>{machine.name}</h1><p>Logbook mesin</p></div>
        <button ref={searchToggle} type="button" className="button button-outline conversation-search-toggle" aria-expanded={searchOpen} aria-controls="conversation-search" onClick={() => searchOpen ? clearSearch(true) : setSearchOpen(true)}>Cari pesan</button>
      </header>
      {searchOpen && <form id="conversation-search" className="conversation-search" role="search" aria-label="Cari pesan mesin" onSubmit={commitSearch} onKeyDown={(event) => { if (event.key === "Escape") { event.preventDefault(); clearSearch(true); } }}>
        <label htmlFor="message-search">Cari teks di mesin ini</label>
        <div className="conversation-search-row">
          <input ref={searchInput} id="message-search" type="search" value={searchText} onChange={(event) => { setSearchText(event.target.value); setSearchError(""); }} aria-describedby="message-search-help" aria-invalid={Boolean(searchError)} />
          <button className="button" type="submit">Cari</button>
          <button className="button button-outline" type="button" onClick={() => clearSearch()}>Reset</button>
          <button className="icon-button" type="button" aria-label="Tutup pencarian" title="Tutup pencarian" onClick={() => clearSearch(true)}><Icon name="close" /></button>
        </div>
        <p id="message-search-help">Seluruh riwayat mesin · Maks. 200 karakter</p>
        <ErrorNotice error={searchError} />
      </form>}
      {query && <p className="conversation-search-status" role="status">{state.loading ? "Mencari pesan…" : state.error ? "Pencarian gagal. Coba lagi di bawah." : `Hasil pencarian: ${query}`}</p>}
      <section className="conversation-timeline" ref={timeline} aria-label="Riwayat pesan" tabIndex={0} aria-busy={state.loading}>
        {state.loading ? <Loading>{query ? "Memuat hasil pencarian…" : "Memuat pesan…"}</Loading> : state.error ? <ErrorNotice error={state.error} onRetry={() => setAttempt((value) => value + 1)} /> : <>
          {state.cursor && <div className="older-messages"><button type="button" className="button button-outline" onClick={loadOlder} disabled={more.loading}>{more.loading ? "Memuat pesan lama…" : query ? "Muat hasil sebelumnya" : "Muat pesan sebelumnya"}</button></div>}
          <ErrorNotice error={more.error} onRetry={loadOlder} />
          {!items.length && <div className="empty-state"><h2>{query ? "Tidak ada pesan yang cocok" : "Belum ada pesan"}</h2><p>{query ? "Coba teks lain atau reset pencarian untuk melihat seluruh riwayat." : "Tulis catatan atau pilih foto untuk memulai logbook mesin ini."}</p></div>}
          <ol className="message-list">
            {items.map((message, index) => {
              const date = dayFormat.format(new Date(message.created_at));
              const previous = items[index - 1];
              const own = user?.id != null && String(message.author.id) === String(user.id);
              return <li key={message.id}>
                {(!previous || dayFormat.format(new Date(previous.created_at)) !== date) && <div className="message-date"><span>{date}</span></div>}
                <article className={`message-entry${own ? " message-own" : ""}`} aria-label={`Pesan ${message.author.name}, ${formatTime(message.created_at)}`}>
                  <header className="message-meta"><strong>{own ? "Anda" : message.author.name}</strong></header>
                  {message.legacy_archive && <div className="legacy-message"><span>Arsip foto · {formatDate(message.legacy_archive.log_date)} · {message.legacy_archive.shift.name}</span><Link to={`/logbooks/${message.legacy_archive.id}`}>Buka arsip lama</Link></div>}
                  {message.text !== "" && <p className="message-text">{message.text}</p>}
                  {message.photos.length > 0 && <div className="message-photos">{message.photos.map((photo) => <MessagePhoto key={photo.id} photo={photo} onOpen={() => setActivePhotoId(photo.id)} />)}</div>}
                  <footer className="message-footer">
                    <time className="message-time" dateTime={message.created_at} title={formatTime(message.created_at)}>{clockFormat.format(new Date(message.created_at))}</time>
                    {own && !message.legacy_archive && <button type="button" className="icon-button message-delete" disabled={deleting} aria-label={`Hapus pesan Anda, ${formatTime(message.created_at)}`} title="Hapus pesan" onClick={(event) => { deleteTrigger.current = event.currentTarget; setDeleteError(null); setDeleteStatus(""); setConfirmDelete(message); }}><Icon name="trash" size={16} /></button>}
                  </footer>
                </article>
              </li>;
            })}
          </ol>
        </>}
      </section>
      <form className="message-composer" onSubmit={send} aria-label="Kirim catatan mesin" aria-busy={draft.sending}>
        {draft.photos.length > 0 && <ul className="draft-photos" aria-label="Foto yang akan dikirim">{draft.photos.map((photo, index) => <li key={photo.url}>
          <img src={photo.url} alt={`Pratinjau foto ${index + 1}`} />
          <button className="draft-photo-remove" type="button" disabled={draft.sending} onClick={() => { release(photo); update(machineId, { photos: draft.photos.filter((item) => item !== photo) }); setSelectionError(""); }} aria-label={`Hapus foto ${index + 1}`} title={`Hapus ${photo.file.name}`}><Icon name="close" size={16} /></button>
        </li>)}</ul>}
        <ErrorNotice error={selectionError} />
        {count > textLimit && <ErrorNotice error="Pesan maksimal 20.000 karakter." />}
        <ErrorNotice error={draft.error} />
        {draft.error && <div className="composer-retry"><p>Draft tetap tersimpan. Jika koneksi terputus, periksa riwayat sebelum mengirim ulang agar pesan tidak ganda.</p><button type="button" className="button button-outline" disabled={state.loading} onClick={() => { seenSent.current = null; setAttempt((value) => value + 1); }}>Muat ulang riwayat</button></div>}
        <div className="composer-row">
          <input ref={fileInput} id="message-photos" type="file" accept="image/jpeg,image/png,image/webp" multiple hidden disabled={draft.sending || draft.photos.length >= 20} onChange={choosePhotos} />
          <button type="button" className="icon-button" onClick={() => fileInput.current.click()} disabled={draft.sending || draft.photos.length >= 20} aria-label="Tambah foto dari galeri" title="Tambah foto dari galeri"><Icon name="plus" /></button>
          <label className="visually-hidden" htmlFor="message-text">Pesan untuk {machine.name}</label>
          <textarea ref={textarea} id="message-text" rows={1} placeholder="Tulis pesan…" value={draft.text} disabled={draft.sending} aria-describedby="message-help" aria-invalid={count > textLimit} onChange={(event) => update(machineId, { text: event.target.value, error: null, sent: null })} />
          <button type="button" className="icon-button" onClick={() => setCameraOpen(true)} disabled={draft.sending || draft.photos.length >= 20} aria-label="Ambil foto dari kamera" title="Ambil foto dari kamera"><Icon name="camera" /></button>
          <button className="icon-button composer-send" type="submit" aria-label={draft.sending ? "Mengirim pesan" : "Kirim pesan"} title={draft.sending ? "Mengirim pesan" : "Kirim pesan"} disabled={draft.sending || count > textLimit || (!draft.text.trim() && !draft.photos.length)}><Icon name="up" /></button>
        </div>
        <div className="composer-footnote"><p id="message-help">{draft.sending ? "Mengirim pesan…" : draft.photos.length ? `${draft.photos.length}/20 foto · Maks. 10 MB/foto` : "Teks, foto, atau keduanya"}</p><span role="status">{draft.sent ? "Pesan terkirim." : count > textLimit - 1000 ? `${count.toLocaleString("id-ID")}/${textLimit.toLocaleString("id-ID")}` : ""}</span></div>
        <span className="visually-hidden">Foto JPG, PNG, WebP. Maksimal 20 foto, 10 MB per foto. Draft tetap ada saat berpindah mesin. Menutup atau memuat ulang tab menghapus draft.</span>
      </form>
      <span className="visually-hidden" role="status">{deleteStatus}</span>
      {confirmDelete && <Dialog title="Hapus pesan ini?" className="message-delete-dialog" onClose={closeDelete} dismissible={!deleting}>
        <p>Pesan Anda di <strong>{machine.name}</strong> pada {formatTime(confirmDelete.created_at)} beserta seluruh fotonya akan dihapus permanen. Tindakan ini tidak dapat dibatalkan.</p>
        <ErrorNotice error={deleteError} />
        <div className="dialog-actions" aria-busy={deleting}>
          <button type="button" className="button button-outline" disabled={deleting} onClick={closeDelete}>Batal</button>
          <button type="button" className="button button-danger" disabled={deleting} onClick={deleteMessage}>{deleting ? "Menghapus pesan…" : deleteError ? "Coba hapus lagi" : "Hapus pesan"}</button>
        </div>
      </Dialog>}
      {cameraOpen && <Camera onClose={() => setCameraOpen(false)} onCapture={handleCameraCapture} onRemove={(photo) => { release(photo); update(machineId, { photos: draft.photos.filter((item) => item !== photo) }); setSelectionError(""); }} photos={draft.photos} maximum={20} />}
      {activePhoto && (
        <PhotoDialog
          title={`Foto ${activePhoto.page_number}${activePhoto.totalInMessage > 1 ? ` dari ${activePhoto.totalInMessage}` : ""}`}
          subtitle={`${activePhoto.messageAuthor} · ${formatDate(activePhoto.messageTime)} ${clockFormat.format(new Date(activePhoto.messageTime))}`}
          caption={activePhoto.messageText || null}
          counterText={`${activeIndex + 1} dari ${allPhotos.length}`}
          photoUrl={viewerPhoto.url}
          loading={viewerPhoto.loading}
          error={viewerPhoto.error}
          onRetry={viewerPhoto.retry}
          hasPrev={activeIndex > 0}
          hasNext={activeIndex < allPhotos.length - 1}
          onPrev={() => setActivePhotoId(allPhotos[activeIndex - 1]?.id)}
          onNext={() => setActivePhotoId(allPhotos[activeIndex + 1]?.id)}
          downloadName={`foto-${activePhoto.id}.${activePhoto.content_type?.split("/")[1] || "jpg"}`}
          onClose={() => setActivePhotoId(null)}
        />
      )}
    </div>
  );
}
