package httpapi

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
)

func (s *Server) rotatePhoto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "use application/json")
		return
	}
	var input struct {
		Degrees int    `json:"degrees"`
		SHA256  string `json:"sha256"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		s.decodeError(w, r, err)
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		problem(w, r, 400, "invalid_json", "expected one JSON object")
		return
	}
	parentID, kind := r.PathValue("logbookID"), "logbook"
	var machineID int64
	if r.PathValue("messageID") != "" {
		kind, parentID = "message", r.PathValue("messageID")
		machineID, err = strconv.ParseInt(r.PathValue("machineID"), 10, 64)
		if err != nil {
			problem(w, r, 400, "validation_error", "invalid machine ID")
			return
		}
	}
	photo, err := s.logbooks.RotatePhoto(r.Context(), currentUser(r), machineID, parentID, kind, r.PathValue("photoID"), input.Degrees, input.SHA256)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, photo)
}
