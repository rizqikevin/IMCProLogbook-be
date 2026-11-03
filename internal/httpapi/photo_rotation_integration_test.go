package httpapi_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"strings"
	"testing"

	"machine-logbook/internal/domain"
)

func TestIntegrationPhotoRotationPersistsPixelsAndPermissions(t *testing.T) {
	app := newIntegrationApp(t)
	owner, reader := app.login(t, "admin"), app.login(t, "operator")
	original := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			original.SetNRGBA(x, y, color.NRGBA{R: uint8(40 + x*70), G: uint8(20 + y*120), B: uint8(x + y*3), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, original); err != nil {
		t.Fatal(err)
	}
	for _, logbook := range []bool{false, true} {
		t.Run(fmt.Sprintf("logbook=%t", logbook), func(t *testing.T) {
			body, contentType := uploadBody(t, logbook, encoded.Bytes())
			path := "/api/v1/machines/1/messages"
			if logbook {
				path = "/api/v1/logbooks"
			}
			created := app.request("POST", path, owner.AccessToken, contentType, body)
			requireStatus(t, created, 201)
			var photo domain.Photo
			if logbook {
				photo = decodeBody[domain.Logbook](t, created).Photos[0]
			} else {
				photo = decodeBody[domain.Message](t, created).Photos[0]
			}
			u, err := url.Parse(photo.URL)
			if err != nil {
				t.Fatal(err)
			}
			endpoint := u.Path + "/rotate"
			payload := fmt.Sprintf(`{"degrees":90,"sha256":%q}`, photo.SHA256)
			requireStatus(t, app.request("POST", endpoint, "", "application/json", strings.NewReader(payload)), 401)
			requireStatus(t, app.request("POST", endpoint, reader.AccessToken, "application/json", strings.NewReader(payload)), 403)
			requireStatus(t, app.request("POST", endpoint, owner.AccessToken, "application/json", strings.NewReader(fmt.Sprintf(`{"degrees":45,"sha256":%q}`, photo.SHA256))), 400)
			before := app.request("GET", photo.URL, owner.AccessToken, "", nil)
			requireStatus(t, before, 200)
			if !bytes.Equal(before.Body.Bytes(), encoded.Bytes()) {
				t.Fatal("rejected rotation changed original bytes")
			}
			rotated := app.request("POST", endpoint, owner.AccessToken, "application/json", strings.NewReader(payload))
			requireStatus(t, rotated, 200)
			saved := decodeBody[domain.Photo](t, rotated)
			if saved.SHA256 == photo.SHA256 || saved.URL == photo.URL || saved.ContentType != "image/png" {
				t.Fatalf("rotation did not refresh photo metadata: %+v", saved)
			}
			requireStatus(t, app.request("POST", endpoint, owner.AccessToken, "application/json", strings.NewReader(payload)), 409)
			for _, fetchURL := range []string{saved.URL, u.Path} {
				response := app.request("GET", fetchURL, owner.AccessToken, "", nil)
				requireStatus(t, response, 200)
				decoded, err := png.Decode(bytes.NewReader(response.Body.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Bounds().Dx() != 2 || decoded.Bounds().Dy() != 3 || int64(response.Body.Len()) != saved.Size {
					t.Fatalf("wrong rotated dimensions/size: %v, %d, %d", decoded.Bounds(), response.Body.Len(), saved.Size)
				}
				for y := range 2 {
					for x := range 3 {
						got := color.NRGBAModel.Convert(decoded.At(1-y, x))
						if got != original.NRGBAAt(x, y) {
							t.Fatalf("pixel (%d,%d): got %v, want %v", x, y, got, original.NRGBAAt(x, y))
						}
					}
				}
			}
		})
	}
}
