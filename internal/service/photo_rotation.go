package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"time"

	"github.com/google/uuid"
	"machine-logbook/internal/domain"
)

// RotatePhoto replaces stored pixels; marker coordinates already refer to EXIF-normalized display pixels.
func (s *Logbooks) RotatePhoto(ctx context.Context, user domain.User, machineID int64, parentID, kind, photoID string, degrees int, expectedSHA string) (domain.Photo, error) {
	if !validUUID(user.ID) {
		return domain.Photo{}, domain.ErrUnauthorized
	}
	if !validUUID(parentID) || !validUUID(photoID) || (kind != "logbook" && kind != "message") || (kind == "message" && machineID <= 0) || (degrees != 90 && degrees != -90 && degrees != 180) {
		return domain.Photo{}, invalid("invalid photo reference or rotation; degrees must be 90, -90, or 180")
	}
	hash, err := hex.DecodeString(expectedSHA)
	if err != nil || len(hash) != sha256.Size {
		return domain.Photo{}, invalid("sha256 must be the current photo hash")
	}
	var original domain.Photo
	if kind == "message" {
		original, err = s.repo.MessagePhoto(ctx, machineID, parentID, photoID)
	} else {
		original, err = s.repo.Photo(ctx, parentID, photoID)
	}
	if err != nil {
		return domain.Photo{}, err
	}
	if user.Role != "admin" && user.ID != original.UploadedBy {
		return domain.Photo{}, domain.ErrForbidden
	}
	if original.SHA256 != expectedSHA {
		return domain.Photo{}, domain.ErrConflict
	}
	if original.Size <= 0 || original.Size > s.limits.MaxPhotoBytes {
		return domain.Photo{}, invalid("photo exceeds rotation byte limit")
	}
	reader, err := s.store.Open(ctx, original.StorageKey)
	if err != nil {
		return domain.Photo{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, s.limits.MaxPhotoBytes+1))
	closeErr := reader.Close()
	if readErr != nil {
		return domain.Photo{}, readErr
	}
	if closeErr != nil {
		return domain.Photo{}, closeErr
	}
	digest := sha256.Sum256(data)
	if int64(len(data)) != original.Size || hex.EncodeToString(digest[:]) != original.SHA256 {
		return domain.Photo{}, invalid("stored photo does not match metadata")
	}
	encoded, contentType, extension, err := s.rotatePixels(data, degrees)
	if err != nil {
		return domain.Photo{}, err
	}
	updated := original
	updated.ContentType, updated.Size = contentType, int64(len(encoded))
	digest = sha256.Sum256(encoded)
	updated.SHA256 = hex.EncodeToString(digest[:])
	updated.StorageKey = kind + "s/" + parentID + "/" + uuid.NewString() + extension
	if err := s.repo.ReserveUploads(ctx, []string{updated.StorageKey}, time.Now().UTC().Add(24*time.Hour)); err != nil {
		return domain.Photo{}, err
	}
	if err := s.store.Put(ctx, updated.StorageKey, bytes.NewReader(encoded), updated.Size, updated.ContentType); err != nil {
		return domain.Photo{}, err
	}
	updated, err = s.repo.ReplaceRotatedPhoto(ctx, user, machineID, parentID, kind, original, updated, degrees)
	if err != nil {
		return domain.Photo{}, err
	}
	if kind == "message" {
		updated.URL = messagePhotoURL(machineID, parentID, photoID)
	} else {
		updated.URL = photoURL(parentID, photoID)
	}
	updated.URL = versionPhotoURL(updated.URL, updated.SHA256)
	return updated, nil
}

func (s *Logbooks) rotatePixels(data []byte, degrees int) ([]byte, string, string, error) {
	if int64(len(data)) > s.limits.MaxPhotoBytes {
		return nil, "", "", invalid("photo exceeds rotation byte limit")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, "", "", invalid("rotation supports valid JPEG, PNG, and WebP images only")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > s.limits.MaxImagePixels/int64(config.Height) {
		return nil, "", "", invalid("photo exceeds rotation pixel limit")
	}
	orientation := 1
	if format == "jpeg" {
		orientation, err = jpegOrientation(data)
		if err != nil {
			return nil, "", "", err
		}
	}
	if err := checkRotationContainer(data, format); err != nil {
		return nil, "", "", err
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", invalid("photo is corrupt or incomplete")
	}
	w, h := config.Width, config.Height
	nw, nh := w, h
	if orientation >= 5 {
		nw, nh = h, w
	}
	ow, oh := nw, nh
	if degrees != 180 {
		ow, oh = nh, nw
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := range h {
		for x := range w {
			u, v := x, y
			switch orientation {
			case 2:
				u = w - 1 - x
			case 3:
				u, v = w-1-x, h-1-y
			case 4:
				v = h - 1 - y
			case 5:
				u, v = y, x
			case 6:
				u, v = h-1-y, x
			case 7:
				u, v = h-1-y, w-1-x
			case 8:
				u, v = y, w-1-x
			}
			switch degrees {
			case 90:
				u, v = nh-1-v, u
			case -90:
				u, v = v, nw-1-u
			case 180:
				u, v = nw-1-u, nh-1-v
			}
			out.Set(u, v, source.At(x, y))
		}
	}
	var encoded bytes.Buffer
	contentType, extension := "image/png", ".png"
	if format == "jpeg" {
		contentType, extension = "image/jpeg", ".jpg"
		err = jpeg.Encode(&encoded, out, &jpeg.Options{Quality: 95})
	} else {
		err = png.Encode(&encoded, out)
	}
	if err != nil {
		return nil, "", "", err
	}
	if int64(encoded.Len()) > s.limits.MaxPhotoBytes {
		return nil, "", "", invalid("rotated photo exceeds byte limit")
	}
	return encoded.Bytes(), contentType, extension, nil
}

func jpegOrientation(data []byte) (int, error) {
	bad := invalid("invalid JPEG EXIF orientation metadata")
	for pos := 2; pos < len(data); {
		if data[pos] != 0xff {
			return 0, bad
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos >= len(data) {
			return 0, bad
		}
		marker := data[pos]
		pos++
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if pos+2 > len(data) {
			return 0, bad
		}
		length := int(binary.BigEndian.Uint16(data[pos:]))
		if length < 2 || length > len(data)-pos {
			return 0, bad
		}
		segment := data[pos+2 : pos+length]
		pos += length
		if marker != 0xe1 || !bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			continue
		}
		tiff := segment[6:]
		if len(tiff) < 8 {
			return 0, bad
		}
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 0, bad
		}
		if order.Uint16(tiff[2:4]) != 42 {
			return 0, bad
		}
		offset := uint64(order.Uint32(tiff[4:8]))
		if offset < 8 || offset+2 > uint64(len(tiff)) {
			return 0, bad
		}
		count := uint64(order.Uint16(tiff[offset : offset+2]))
		if offset+2+count*12 > uint64(len(tiff)) {
			return 0, bad
		}
		for n := range count {
			entry := tiff[offset+2+n*12 : offset+2+(n+1)*12]
			if order.Uint16(entry[:2]) != 0x112 {
				continue
			}
			if order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) != 1 {
				return 0, bad
			}
			value := int(order.Uint16(entry[8:10]))
			if value < 1 || value > 8 {
				return 0, bad
			}
			return value, nil
		}
	}
	return 1, nil
}

func versionPhotoURL(url, hash string) string {
	if hash == "" {
		return url
	}
	return url + "?v=" + hash
}

func checkRotationContainer(data []byte, format string) error {
	bad := invalid("rotation does not support animated images or PNG/WebP EXIF metadata")
	if format == "png" {
		for pos := 8; pos < len(data); {
			if len(data)-pos < 12 {
				return bad
			}
			size := uint64(binary.BigEndian.Uint32(data[pos : pos+4]))
			if size+12 > uint64(len(data)-pos) {
				return bad
			}
			kind := string(data[pos+4 : pos+8])
			if kind == "acTL" || kind == "eXIf" {
				return bad
			}
			pos += int(size) + 12
		}
	}
	if format == "webp" {
		if len(data) < 12 {
			return bad
		}
		for pos := 12; pos < len(data); {
			if len(data)-pos < 8 {
				return bad
			}
			size := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			padded := size + size%2
			if padded+8 > uint64(len(data)-pos) {
				return bad
			}
			kind := string(data[pos : pos+4])
			if kind == "ANIM" || kind == "ANMF" || kind == "EXIF" {
				return bad
			}
			if kind == "VP8X" && size > 0 && data[pos+8]&0x0a != 0 {
				return bad
			}
			pos += int(padded) + 8
		}
	}
	return nil
}
