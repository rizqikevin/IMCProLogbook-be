package httpapi_test

import (
	"strings"
	"testing"

	"machine-logbook/internal/domain"
)

func TestIntegrationPhotoMarkersPermissionsAndPersistence(t *testing.T) {
	app := newIntegrationApp(t)
	owner, reader := app.login(t, "admin"), app.login(t, "operator")
	_, jpeg := integrationImages(t)
	for _, legacy := range []bool{false, true} {
		body, contentType := uploadBody(t, legacy, jpeg)
		path := "/api/v1/machines/1/messages"
		if legacy {
			path = "/api/v1/logbooks"
		}
		response := app.request("POST", path, owner.AccessToken, contentType, body)
		requireStatus(t, response, 201)
		var photo domain.Photo
		if legacy {
			photo = decodeBody[domain.Logbook](t, response).Photos[0]
		} else {
			photo = decodeBody[domain.Message](t, response).Photos[0]
		}
		endpoint := photo.URL + "/markers"
		requireStatus(t, app.request("GET", endpoint, "", "", nil), 401)
		listing := app.request("GET", endpoint, reader.AccessToken, "", nil)
		requireStatus(t, listing, 200)
		type markerPage struct {
			Items   []domain.PhotoMarker `json:"items"`
			CanEdit bool                 `json:"can_edit"`
		}
		if decodeBody[markerPage](t, listing).CanEdit {
			t.Fatal("reader can edit")
		}
		requireStatus(t, app.request("POST", endpoint, reader.AccessToken, "application/json", strings.NewReader(`{"x":0.2,"y":0.7}`)), 403)
		for _, payload := range []string{`{}`, `{"x":null,"y":0}`, `{"x":-0.1,"y":0}`, `{"x":0,"y":1.1}`, `{"x":0,"y":0} {}`} {
			requireStatus(t, app.request("POST", endpoint, owner.AccessToken, "application/json", strings.NewReader(payload)), 400)
		}
		created := app.request("POST", endpoint, owner.AccessToken, "application/json", strings.NewReader(`{"x":0,"y":1}`))
		requireStatus(t, created, 201)
		marker := decodeBody[domain.PhotoMarker](t, created)
		listing = app.request("GET", endpoint, reader.AccessToken, "", nil)
		requireStatus(t, listing, 200)
		page := decodeBody[markerPage](t, listing)
		if len(page.Items) != 1 || page.Items[0].ID != marker.ID || page.Items[0].X != 0 || page.Items[0].Y != 1 {
			t.Fatalf("marker not persisted: %+v", page)
		}
		if !legacy {
			wrong := strings.Replace(endpoint, "/machines/1/", "/machines/2/", 1)
			requireStatus(t, app.request("GET", wrong, owner.AccessToken, "", nil), 404)
			requireStatus(t, app.request("POST", wrong, owner.AccessToken, "application/json", strings.NewReader(`{"x":0,"y":0}`)), 404)
		}
		requireStatus(t, app.request("DELETE", endpoint+"/"+marker.ID, reader.AccessToken, "", nil), 403)
		requireStatus(t, app.request("DELETE", endpoint+"/"+marker.ID, owner.AccessToken, "", nil), 204)
		listing = app.request("GET", endpoint, owner.AccessToken, "", nil)
		if page := decodeBody[markerPage](t, listing); len(page.Items) != 0 || !page.CanEdit {
			t.Fatalf("delete not persisted: %+v", page)
		}
	}
}
