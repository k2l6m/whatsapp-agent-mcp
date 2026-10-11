package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

func sendWhatsAppMessage(client *whatsmeow.Client, recipient string, message string, mediaPath string, quotedMessageID string, quotedParticipant string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	var recipientJID types.JID
	var err error
	if strings.Contains(recipient, "@") {
		recipientJID, err = types.ParseJID(recipient)
		if err != nil {
			return false, fmt.Sprintf("Error parsing JID: %v", err)
		}
	} else {
		recipientJID = types.JID{User: recipient, Server: "s.whatsapp.net"}
	}

	msg := &waProto.Message{}

	// Check if this is a reply (quoted message)
	if mediaPath == "" && quotedMessageID != "" {
		msg.ExtendedTextMessage = &waProto.ExtendedTextMessage{
			Text: proto.String(message),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:    proto.String(quotedMessageID),
				Participant: proto.String(quotedParticipant),
			},
		}
	} else if mediaPath != "" {
		mediaData, err := os.ReadFile(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error reading media file: %v", err)
		}

		fileExt := strings.ToLower(mediaPath[strings.LastIndex(mediaPath, ".")+1:])
		var mediaType whatsmeow.MediaType
		var mimeType string

		switch fileExt {
		case "jpg", "jpeg":
			mediaType, mimeType = whatsmeow.MediaImage, "image/jpeg"
		case "png":
			mediaType, mimeType = whatsmeow.MediaImage, "image/png"
		case "webp":
			mediaType, mimeType = whatsmeow.MediaImage, "image/webp"
		case "ogg":
			mediaType, mimeType = whatsmeow.MediaAudio, "audio/ogg; codecs=opus"
		case "mp4", "mov":
			mediaType, mimeType = whatsmeow.MediaVideo, "video/mp4"
		default:
			mediaType, mimeType = whatsmeow.MediaDocument, "application/octet-stream"
		}

		resp, err := client.Upload(context.Background(), mediaData, mediaType)
		if err != nil {
			return false, fmt.Sprintf("Error uploading media: %v", err)
		}

		switch mediaType {
		case whatsmeow.MediaImage:
			msg.ImageMessage = &waProto.ImageMessage{Caption: proto.String(message), Mimetype: proto.String(mimeType), URL: &resp.URL, DirectPath: &resp.DirectPath, MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256, FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength}
		case whatsmeow.MediaAudio:
			sec, wave, _ := analyzeOggOpus(mediaData)
			msg.AudioMessage = &waProto.AudioMessage{Mimetype: proto.String(mimeType), URL: &resp.URL, DirectPath: &resp.DirectPath, MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256, FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength, Seconds: proto.Uint32(sec), PTT: proto.Bool(true), Waveform: wave}
		case whatsmeow.MediaVideo:
			msg.VideoMessage = &waProto.VideoMessage{Caption: proto.String(message), Mimetype: proto.String(mimeType), URL: &resp.URL, DirectPath: &resp.DirectPath, MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256, FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength}
		case whatsmeow.MediaDocument:
			cleanName := filepath.Base(mediaPath)
			msg.DocumentMessage = &waProto.DocumentMessage{Title: proto.String(cleanName), FileName: proto.String(cleanName), Caption: proto.String(message), Mimetype: proto.String(mimeType), URL: &resp.URL, DirectPath: &resp.DirectPath, MediaKey: resp.MediaKey, FileEncSHA256: resp.FileEncSHA256, FileSHA256: resp.FileSHA256, FileLength: &resp.FileLength}
		}
	} else {
		msg.Conversation = proto.String(message)
	}

	_, err = client.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return false, fmt.Sprintf("Error sending message: %v", err)
	}
	return true, fmt.Sprintf("Message sent to %s", recipient)
}

func handleMessage(client *whatsmeow.Client, messageStore *MessageStore, msg *events.Message, logger waLog.Logger) {
	chatJID := msg.Info.Chat.String()
	sender := msg.Info.Sender.User
	name := GetChatName(client, messageStore, msg.Info.Chat, chatJID, nil, sender, logger)

	_ = messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	content := extractTextContent(msg.Message)
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	if content == "" && mediaType == "" {
		return
	}

	_ = messageStore.StoreMessage(msg.Info.ID, chatJID, sender, content, msg.Info.Timestamp, msg.Info.IsFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength)
}

func GetChatName(client *whatsmeow.Client, messageStore *MessageStore, jid types.JID, chatJID string, conversation interface{}, sender string, logger waLog.Logger) string {
	var existingName string
	if err := messageStore.db.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&existingName); err == nil && existingName != "" {
		return existingName
	}

	var name string
	if jid.Server == "g.us" {
		if conversation != nil {
			v := reflect.ValueOf(conversation)
			if v.Kind() == reflect.Ptr && !v.IsNil() {
				v = v.Elem()
				if dn := v.FieldByName("DisplayName"); dn.IsValid() && dn.Kind() == reflect.Ptr && !dn.IsNil() {
					name = dn.Elem().String()
				}
			}
		}
		if name == "" {
			if groupInfo, err := client.GetGroupInfo(context.Background(), jid); err == nil && groupInfo.Name != "" {
				name = groupInfo.Name
			} else {
				name = fmt.Sprintf("Group %s", jid.User)
			}
		}
	} else {
		if contact, err := client.Store.Contacts.GetContact(context.Background(), jid); err == nil && contact.FullName != "" {
			name = contact.FullName
		} else if sender != "" {
			name = sender
		} else {
			name = jid.User
		}
	}
	return name
}

func handleHistorySync(client *whatsmeow.Client, messageStore *MessageStore, historySync *events.HistorySync, logger waLog.Logger) {
	for _, conversation := range historySync.Data.Conversations {
		if conversation.ID == nil {
			continue
		}
		chatJID := *conversation.ID
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			continue
		}
		name := GetChatName(client, messageStore, jid, chatJID, conversation, "", logger)
		if len(conversation.Messages) > 0 {
			latest := conversation.Messages[0]
			if latest != nil && latest.Message != nil && latest.Message.GetMessageTimestamp() != 0 {
				_ = messageStore.StoreChat(chatJID, name, time.Unix(int64(latest.Message.GetMessageTimestamp()), 0))
			}
		}
	}
}

func main() {
	logger := waLog.Stdout("Client", "INFO", true)
	dbLog := waLog.Stdout("Database", "INFO", true)
	_ = os.MkdirAll("store", 0755)

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		deviceStore = container.NewDevice()
	}

	client := whatsmeow.NewClient(deviceStore, logger)
	messageStore, err := NewMessageStore()
	if err != nil {
		logger.Errorf("Failed to initialize message store: %v", err)
		return
	}
	defer messageStore.Close()

	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			handleMessage(client, messageStore, v, logger)
		case *events.HistorySync:
			handleHistorySync(client, messageStore, v, logger)
		case *events.Connected:
			logger.Infof("Connected to WhatsApp")
		case *events.LoggedOut:
			logger.Warnf("Device logged out")
		}
	})

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		_ = client.Connect()
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "success" {
				break
			}
		}
	} else {
		_ = client.Connect()
	}

	time.Sleep(2 * time.Second)
	startRESTServer(client, messageStore, 8080)

	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)
	<-exitChan
	client.Disconnect()
}