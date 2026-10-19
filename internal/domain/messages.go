package domain

import "time"

type MessageAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LegacyArchive struct {
	ID      string `json:"id"`
	LogDate string `json:"log_date"`
	Shift   Shift  `json:"shift"`
}

type Message struct {
	ID            string         `json:"id"`
	MachineID     int64          `json:"machine_id"`
	Text          string         `json:"text"`
	Author        MessageAuthor  `json:"author"`
	CreatedAt     time.Time      `json:"created_at"`
	EditedAt      *time.Time     `json:"edited_at"`
	Photos        []Photo        `json:"photos"`
	LegacyArchive *LegacyArchive `json:"legacy_archive,omitempty"`
}

type MessageCursor struct {
	MachineID int64     `json:"machine_id"`
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	QueryHash string    `json:"query_hash,omitempty"`
}

type MessageFilter struct {
	MachineID int64
	Cursor    *MessageCursor
	Limit     int
	Query     string
}

type MessagePage struct {
	Items      []Message `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}
