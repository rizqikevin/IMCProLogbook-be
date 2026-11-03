package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"reflect"
	"testing"
	"time"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/storage"
)

func TestRotatePixelsDirections(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(y*3 + x + 1), A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		degrees, width, height int
		pixels                 []uint8
	}{
		{90, 2, 3, []uint8{4, 1, 5, 2, 6, 3}}, {-90, 2, 3, []uint8{3, 6, 2, 5, 1, 4}}, {180, 3, 2, []uint8{6, 5, 4, 3, 2, 1}},
	} {
		data, mime, ext, err := testLogbooks(nil, nil).rotatePixels(input.Bytes(), tc.degrees)
		if err != nil || mime != "image/png" || ext != ".png" {
			t.Fatalf("rotation: %s %s %v", mime, ext, err)
		}
		got, _, err := image.Decode(bytes.NewReader(data))
		if err != nil || got.Bounds().Dx() != tc.width || got.Bounds().Dy() != tc.height {
			t.Fatalf("dimensions: %v", err)
		}
		for i, want := range tc.pixels {
			r, _, _, _ := got.At(i%tc.width, i/tc.width).RGBA()
			if uint8(r>>8) != want {
				t.Fatalf("%d pixel %d: %d != %d", tc.degrees, i, r>>8, want)
			}
		}
	}
}

func withTestEXIF(data []byte, orientation uint16, order binary.ByteOrder) []byte {
	tiff := make([]byte, 26)
	copy(tiff, "II")
	if order == binary.BigEndian {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], orientation)
	segment := append([]byte("Exif\x00\x00"), tiff...)
	out := append([]byte{}, data[:2]...)
	out = append(out, 0xff, 0xe1, byte((len(segment)+2)>>8), byte(len(segment)+2))
	out = append(out, segment...)
	return append(out, data[2:]...)
}

func TestRotateJPEGEXIF(t *testing.T) {
	data := imageBytes(t, "jpeg", 3, 2)
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := uint16(1); orientation <= 8; orientation++ {
			input := withTestEXIF(data, orientation, order)
			got, err := jpegOrientation(input)
			if err != nil || got != int(orientation) {
				t.Fatalf("EXIF %d: %d %v", orientation, got, err)
			}
			encoded, _, _, err := testLogbooks(nil, nil).rotatePixels(input, 90)
			if err != nil {
				t.Fatal(err)
			}
			config, _, err := image.DecodeConfig(bytes.NewReader(encoded))
			w, h := 2, 3
			if orientation >= 5 {
				w, h = 3, 2
			}
			if err != nil || config.Width != w || config.Height != h {
				t.Fatalf("EXIF dimensions %d: %+v %v", orientation, config, err)
			}
			if got, err := jpegOrientation(encoded); err != nil || got != 1 {
				t.Fatalf("output retains EXIF: %d %v", got, err)
			}
		}
	}
	if _, err := jpegOrientation(withTestEXIF(data, 9, binary.LittleEndian)); err == nil {
		t.Fatal("invalid EXIF accepted")
	}
}

func TestRotateRejectsInvalidAndUnsafeImages(t *testing.T) {
	service := testLogbooks(nil, nil)
	for _, data := range [][]byte{[]byte("bad"), imageBytes(t, "gif", 2, 2), imageBytes(t, "png", 11, 10), imageBytes(t, "png", 2, 2)[:30]} {
		if _, _, _, err := service.rotatePixels(data, 90); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	for _, kind := range []string{"acTL", "eXIf"} {
		data := append(make([]byte, 8), 0, 0, 0, 0)
		data = append(data, []byte(kind)...)
		data = append(data, 0, 0, 0, 0)
		if checkRotationContainer(data, "png") == nil {
			t.Fatalf("accepted %s", kind)
		}
	}
	for _, kind := range []string{"ANIM", "ANMF", "EXIF"} {
		data := append(make([]byte, 12), []byte(kind)...)
		data = append(data, 0, 0, 0, 0)
		if checkRotationContainer(data, "webp") == nil {
			t.Fatalf("accepted %s", kind)
		}
	}
}

type rotationRepoStub struct {
	Repository
	photo                  domain.Photo
	events                 *[]string
	reserveErr, replaceErr error
}

func (r *rotationRepoStub) Photo(context.Context, string, string) (domain.Photo, error) {
	return r.photo, nil
}
func (r *rotationRepoStub) MessagePhoto(context.Context, int64, string, string) (domain.Photo, error) {
	return r.photo, nil
}
func (r *rotationRepoStub) ReserveUploads(context.Context, []string, time.Time) error {
	*r.events = append(*r.events, "reserve")
	return r.reserveErr
}
func (r *rotationRepoStub) ReplaceRotatedPhoto(_ context.Context, _ domain.User, _ int64, _, _ string, old, next domain.Photo, _ int) (domain.Photo, error) {
	*r.events = append(*r.events, "replace")
	if r.replaceErr != nil {
		return domain.Photo{}, r.replaceErr
	}
	if old.StorageKey != r.photo.StorageKey || old.SHA256 != r.photo.SHA256 {
		return domain.Photo{}, domain.ErrConflict
	}
	r.photo = next
	return next, nil
}

type rotationStoreStub struct {
	storage.Store
	data   []byte
	events *[]string
	putErr error
}

func (s *rotationStoreStub) Open(context.Context, string) (io.ReadCloser, error) {
	*s.events = append(*s.events, "open")
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s *rotationStoreStub) Put(context.Context, string, io.Reader, int64, string) error {
	*s.events = append(*s.events, "put")
	return s.putErr
}

func TestRotatePhotoAuthorizationAndFailures(t *testing.T) {
	failure := errors.New("injected failure")
	for _, kind := range []string{"logbook", "message"} {
		for _, stage := range []string{"success", "admin", "forbidden", "stale", "reserve", "put", "replace"} {
			t.Run(kind+"/"+stage, func(t *testing.T) {
				data := imageBytes(t, "png", 3, 2)
				hash := sha256.Sum256(data)
				original := domain.Photo{ID: testBookID, UploadedBy: testUserID, StorageKey: "old.png", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data))}
				events := []string{}
				repo := &rotationRepoStub{photo: original, events: &events}
				store := &rotationStoreStub{data: data, events: &events}
				user := domain.User{ID: testUserID}
				expected := original.SHA256
				want := []string{"open", "reserve", "put", "replace"}
				switch stage {
				case "admin":
					user.ID = testBookID
					user.Role = "admin"
				case "forbidden":
					user.ID = testBookID
					want = []string{}
				case "stale":
					expected = hex.EncodeToString(make([]byte, 32))
					want = []string{}
				case "reserve":
					repo.reserveErr = failure
					want = []string{"open", "reserve"}
				case "put":
					store.putErr = failure
					want = []string{"open", "reserve", "put"}
				case "replace":
					repo.replaceErr = domain.ErrConflict
				}
				photo, err := testLogbooks(repo, store).RotatePhoto(context.Background(), user, 1, testBookID, kind, testBookID, 90, expected)
				if stage == "success" || stage == "admin" {
					if err != nil || photo.StorageKey == original.StorageKey || photo.SHA256 == original.SHA256 || photo.URL == "" {
						t.Fatalf("result: %+v %v", photo, err)
					}
				} else {
					if err == nil || !reflect.DeepEqual(repo.photo, original) {
						t.Fatalf("failure changed original: %+v %v", repo.photo, err)
					}
					if stage == "forbidden" && !errors.Is(err, domain.ErrForbidden) {
						t.Fatal(err)
					}
					if stage == "stale" && !errors.Is(err, domain.ErrConflict) {
						t.Fatal(err)
					}
				}
				if !reflect.DeepEqual(events, want) {
					t.Fatalf("events %v != %v", events, want)
				}
			})
		}
	}
}
