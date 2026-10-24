package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

func machinePathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("machineID"), 10, 64)
	if err != nil || id <= 0 {
		return 0, &domain.ValidationError{Message: "machine_id must be a positive integer"}
	}
	return id, nil
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	machineID, err := machinePathID(r)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		problem(w, r, 400, "validation_error", "invalid query string")
		return
	}
	for key, values := range query {
		if (key != "cursor" && key != "limit" && key != "q") || len(values) != 1 || (key == "limit" && values[0] == "") {
			problem(w, r, 400, "validation_error", "invalid or repeated query parameter: "+key)
			return
		}
	}
	filter := domain.MessageFilter{MachineID: machineID, Limit: 20, Query: query.Get("q")}
	if raw := query.Get("limit"); raw != "" {
		filter.Limit, err = strconv.Atoi(raw)
		if err != nil || filter.Limit < 1 || filter.Limit > 100 {
			problem(w, r, 400, "validation_error", "limit must be between 1 and 100")
			return
		}
	}
	filter.Cursor, err = service.ParseMessageCursor(query.Get("cursor"))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	page, err := s.logbooks.ListMessages(r.Context(), filter)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	machineID, err := machinePathID(r)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	uploads, err := s.multipartFields(w, r, map[string]bool{"text": true}, false)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	text := ""
	if values := r.MultipartForm.Value["text"]; len(values) > 0 {
		text = values[0]
	}
	message, err := s.logbooks.CreateMessage(r.Context(), currentUser(r), machineID, text, uploads)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, message)
}

func (s *Server) messagePhoto(w http.ResponseWriter, r *http.Request) {
	machineID, err := machinePathID(r)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	photo, body, err := s.logbooks.OpenMessagePhoto(r.Context(), machineID, r.PathValue("messageID"), r.PathValue("photoID"))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	defer body.Close()
	s.servePhoto(w, r, photo, body)
}
func (s *Server) validateMarkerPath(r *http.Request) error {
	if r.PathValue("logbookID") != "" {
		return nil
	}
	machineID, err := machinePathID(r)
	if err != nil {
		return err
	}
	return s.logbooks.ValidateMarkerMessagePhoto(r.Context(), machineID, r.PathValue("messageID"), r.PathValue("photoID"))
}
func (s *Server) listPhotoMarkers(w http.ResponseWriter, r *http.Request) {
	if err := s.validateMarkerPath(r); err != nil {
		s.failure(w, r, err)
		return
	}
	parentID, parentKind := r.PathValue("messageID"), "message"
	if r.PathValue("logbookID") != "" {
		parentID, parentKind = r.PathValue("logbookID"), "logbook"
	}
	markers, err := s.logbooks.ListPhotoMarkers(r.Context(), r.PathValue("photoID"), parentID, parentKind)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	canEdit, err := s.logbooks.PhotoMarkerCanEdit(r.Context(), currentUser(r), r.PathValue("photoID"), parentID, parentKind)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": markers, "can_edit": canEdit})
}
func (s *Server) addPhotoMarker(w http.ResponseWriter, r *http.Request) {
	if err := s.validateMarkerPath(r); err != nil {
		s.failure(w, r, err)
		return
	}
	var input struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		s.decodeError(w, r, err)
		return
	}
	if dec.Decode(new(any)) != io.EOF {
		s.failure(w, r, &domain.ValidationError{Message: "request must contain one JSON object"})
		return
	}
	if input.X == nil || input.Y == nil {
		s.failure(w, r, &domain.ValidationError{Message: "x and y are required"})
		return
	}
	parentID, parentKind := r.PathValue("messageID"), "message"
	if r.PathValue("logbookID") != "" {
		parentID, parentKind = r.PathValue("logbookID"), "logbook"
	}
	marker, err := s.logbooks.AddPhotoMarker(r.Context(), currentUser(r), r.PathValue("photoID"), parentID, parentKind, *input.X, *input.Y)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, marker)
}
func (s *Server) deletePhotoMarker(w http.ResponseWriter, r *http.Request) {
	if err := s.validateMarkerPath(r); err != nil {
		s.failure(w, r, err)
		return
	}
	parentID, parentKind := r.PathValue("messageID"), "message"
	if r.PathValue("logbookID") != "" {
		parentID, parentKind = r.PathValue("logbookID"), "logbook"
	}
	if err := s.logbooks.DeletePhotoMarker(r.Context(), currentUser(r), r.PathValue("photoID"), r.PathValue("markerID"), parentID, parentKind); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteMessage(w http.ResponseWriter, r *http.Request) {
	machineID, err := machinePathID(r)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	if err := s.logbooks.DeleteMessage(r.Context(), currentUser(r), machineID, r.PathValue("messageID")); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) editMessage(w http.ResponseWriter, r *http.Request) {
	machineID, err := machinePathID(r)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "use application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.decodeError(w, r, err)
		return
	}
	if !utf8.Valid(body) {
		problem(w, r, http.StatusBadRequest, "invalid_json", "expected valid UTF-8 JSON")
		return
	}
	var input struct {
		Text *string `json:"text"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		s.decodeError(w, r, err)
		return
	}
	if dec.Decode(new(any)) != io.EOF || input.Text == nil {
		problem(w, r, http.StatusBadRequest, "invalid_json", "expected one JSON object with text")
		return
	}
	message, err := s.logbooks.EditMessage(r.Context(), currentUser(r), machineID, r.PathValue("messageID"), *input.Text)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, message)
}
