package service

import (
	"context"
	"encoding/json"
	"testing"

	"machine-logbook/internal/domain"
)

type pushMessageRepo struct {
	*messageRepoStub
}

func (r *pushMessageRepo) ActiveUserIDs(context.Context, string) ([]string, error) {
	return []string{testUserID, "recipient"}, nil
}

func TestCreateMessagePushPayload(t *testing.T) {
	repo := &pushMessageRepo{&messageRepoStub{
		logbookRepoStub: &logbookRepoStub{},
		createMessage:   func(_ context.Context, message domain.Message) (domain.Message, error) { return message, nil },
	}}
	books := testLogbooks(repo, &storeStub{})
	push := &PushService{jobs: make(chan pushJob, 2), stop: make(chan struct{})}
	books.SetPushService(push)
	message, err := books.CreateMessage(context.Background(), domain.User{ID: testUserID}, 2, "private message text", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(push.jobs) != 1 {
		t.Fatalf("got %d jobs, want only recipient", len(push.jobs))
	}
	job := <-push.jobs
	if job.userID != "recipient" {
		t.Fatalf("wrong recipient: %s", job.userID)
	}
	var payload map[string]string
	if err := json.Unmarshal(job.payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 2 || payload["url"] != "/machines/2" || payload["tag"] != "message-"+message.ID {
		t.Fatalf("incorrect notification payload: %s", job.payload)
	}
}
