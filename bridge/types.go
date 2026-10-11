package main

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow"
)

type Message struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	IsFromMe  bool      `json:"is_from_me"`
	MediaType string    `json:"media_type"`
	Filename  string    `json:"filename"`
}

type MessageStore struct {
	db *sql.DB
}

type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type SendMessageRequest struct {
	Recipient         string `json:"recipient"`
	Message           string `json:"message"`
	MediaPath         string `json:"media_path,omitempty"`
	QuotedMessageID   string `json:"quoted_message_id,omitempty"`
	QuotedParticipant string `json:"quoted_participant,omitempty"`
}

type DownloadMediaRequest struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
}

type DownloadMediaResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

type Contact struct {
	JID      string `json:"jid"`
	Name     string `json:"name"`
	PushName string `json:"push_name"`
}

type MediaDownloader struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileLength    uint64
	FileSHA256    []byte
	FileEncSHA256 []byte
	MediaType     whatsmeow.MediaType
}

func (d *MediaDownloader) GetDirectPath() string { return d.DirectPath }
func (d *MediaDownloader) GetURL() string { return d.URL }
func (d *MediaDownloader) GetMediaKey() []byte { return d.MediaKey }
func (d *MediaDownloader) GetFileLength() uint64 { return d.FileLength }
func (d *MediaDownloader) GetFileSHA256() []byte { return d.FileSHA256 }
func (d *MediaDownloader) GetFileEncSHA256() []byte { return d.FileEncSHA256 }
func (d *MediaDownloader) GetMediaType() whatsmeow.MediaType { return d.MediaType }