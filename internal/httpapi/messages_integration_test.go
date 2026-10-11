package httpapi_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"

	"machine-logbook/internal/domain"
)

func TestIntegrationMachineMessagesTextPhotosAndHistory(t *testing.T) {
	app := newIntegrationApp(t)
	login := app.login(t, "operator")
	png, jpeg := integrationImages(t)
	archiveBody, archiveType := uploadBody(t, true, jpeg)
	archiveResponse := app.request("POST", "/api/v1/logbooks", login.AccessToken, archiveType, archiveBody)
	requireStatus(t, archiveResponse, http.StatusCreated)
	archive := decodeBody[domain.Logbook](t, archiveResponse)
	var sent []domain.Message
	for _, tc := range []struct {
		text   string
		photos [][]byte
	}{{"  Periksa motor\nbaris dua  ", nil}, {"", [][]byte{png}}, {"Foto kondisi", [][]byte{jpeg, png}}} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		if tc.text != "" {
			if err := form.WriteField("text", tc.text); err != nil {
				t.Fatal(err)
			}
		}
		for _, data := range tc.photos {
			part, err := form.CreateFormFile("photos", "photo.bin")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		response := app.request("POST", "/api/v1/machines/1/messages", login.AccessToken, form.FormDataContentType(), &body)
		requireStatus(t, response, http.StatusCreated)
		message := decodeBody[domain.Message](t, response)
		if message.Text != tc.text || message.Author.ID != login.User.ID || message.Author.Name != login.User.Name || message.CreatedAt.IsZero() || message.Photos == nil || len(message.Photos) != len(tc.photos) || message.LegacyArchive != nil {
			t.Fatalf("wrong message: %+v", message)
		}
		for i, photo := range message.Photos {
			if photo.PageNumber != i+1 || photo.UploadedBy != login.User.ID {
				t.Fatalf("wrong photo metadata: %+v", photo)
			}
			requireStatus(t, app.request("GET", photo.URL, "", "", nil), 401)
			response := app.request("GET", photo.URL, login.AccessToken, "", nil)
			requireStatus(t, response, 200)
			if !bytes.Equal(response.Body.Bytes(), tc.photos[i]) {
				t.Fatal("photo bytes changed")
			}
			head := app.request("HEAD", photo.URL, login.AccessToken, "", nil)
			requireStatus(t, head, 200)
			if head.Body.Len() != 0 || head.Header().Get("Content-Length") != response.Header().Get("Content-Length") {
				t.Fatal("HEAD does not match photo")
			}
		}
		sent = append(sent, message)
	}
	response := app.request("GET", "/api/v1/machines/1/messages?limit=2", login.AccessToken, "", nil)
	requireStatus(t, response, 200)
	page := decodeBody[domain.MessagePage](t, response)
	if len(page.Items) != 2 || page.Items[0].ID != sent[1].ID || page.Items[1].ID != sent[2].ID || page.NextCursor == "" {
		t.Fatalf("newest page: %+v", page)
	}
	olderResponse := app.request("GET", "/api/v1/machines/1/messages?limit=2&cursor="+page.NextCursor, login.AccessToken, "", nil)
	requireStatus(t, olderResponse, 200)
	older := decodeBody[domain.MessagePage](t, olderResponse)
	if len(older.Items) != 2 || older.Items[0].ID != archive.ID || older.Items[1].ID != sent[0].ID || older.NextCursor != "" {
		t.Fatalf("older page: %+v", older)
	}
	legacy := older.Items[0]
	if legacy.Text != "" || legacy.LegacyArchive == nil || legacy.LegacyArchive.ID != archive.ID || !legacy.CreatedAt.Equal(archive.CreatedAt) || legacy.Author.ID != archive.CreatedBy || len(legacy.Photos) != 1 || legacy.Photos[0].URL != archive.Photos[0].URL {
		t.Fatalf("history changed: %+v", legacy)
	}
	requireStatus(t, app.request("GET", legacy.Photos[0].URL, login.AccessToken, "", nil), 200)
	other := app.request("GET", "/api/v1/machines/2/messages", login.AccessToken, "", nil)
	requireStatus(t, other, 200)
	if page := decodeBody[domain.MessagePage](t, other); page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("empty machine: %+v", page)
	}
	requireStatus(t, app.request("GET", "/api/v1/machines/2/messages?cursor="+page.NextCursor, login.AccessToken, "", nil), 400)
	search := app.request("GET", "/api/v1/machines/1/messages?q=fOtO", login.AccessToken, "", nil)
	requireStatus(t, search, 200)
	matched := decodeBody[domain.MessagePage](t, search)
	if len(matched.Items) != 1 || matched.Items[0].ID != sent[2].ID || len(matched.Items[0].Photos) != 2 {
		t.Fatalf("search lost matching photos: %+v", matched)
	}
	search = app.request("GET", "/api/v1/machines/1/messages?q=o&limit=1", login.AccessToken, "", nil)
	requireStatus(t, search, 200)
	matched = decodeBody[domain.MessagePage](t, search)
	if len(matched.Items) != 1 || matched.Items[0].ID != sent[2].ID || matched.NextCursor == "" {
		t.Fatalf("search page: %+v", matched)
	}
	for _, path := range []string{
		"/api/v1/machines/1/messages?cursor=" + matched.NextCursor,
		"/api/v1/machines/1/messages?q=other&cursor=" + matched.NextCursor,
		"/api/v1/machines/2/messages?q=o&cursor=" + matched.NextCursor,
		"/api/v1/machines/1/messages?q=o&cursor=" + page.NextCursor,
	} {
		requireStatus(t, app.request("GET", path, login.AccessToken, "", nil), 400)
	}
	search = app.request("GET", "/api/v1/machines/1/messages?q="+url.QueryEscape(" o ")+"&limit=1&cursor="+matched.NextCursor, login.AccessToken, "", nil)
	requireStatus(t, search, 200)
	matched = decodeBody[domain.MessagePage](t, search)
	if len(matched.Items) != 1 || matched.Items[0].ID != sent[0].ID || matched.NextCursor != "" {
		t.Fatalf("older search page: %+v", matched)
	}
	cleared := app.request("GET", "/api/v1/machines/1/messages?q=%20%09", login.AccessToken, "", nil)
	requireStatus(t, cleared, 200)
	restored := decodeBody[domain.MessagePage](t, cleared)
	if len(restored.Items) != 4 || restored.Items[0].ID != archive.ID || restored.Items[2].ID != sent[1].ID {
		t.Fatalf("clear did not restore history and photo-only message: %+v", restored)
	}
}
