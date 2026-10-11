package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"
)

func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int) {
	http.HandleFunc("/api/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		if req.Recipient == "" || (req.Message == "" && req.MediaPath == "") {
			http.Error(w, "Recipient and message/media path are required", http.StatusBadRequest)
			return
		}

		success, message := sendWhatsAppMessage(client, req.Recipient, req.Message, req.MediaPath, req.QuotedMessageID, req.QuotedParticipant)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(SendMessageResponse{Success: success, Message: message})
	})

	http.HandleFunc("/api/contacts/search", func(w http.ResponseWriter, r *http.Request) {
		query := strings.ToLower(r.URL.Query().Get("query"))
		if query == "" {
			query = strings.ToLower(r.URL.Query().Get("name"))
		}

		db, err := sql.Open("sqlite3", "file:store/whatsapp.db?_foreign_keys=on")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		rows, err := db.Query(`
			SELECT their_jid, full_name, push_name
			FROM whatsmeow_contacts
			WHERE lower(full_name) LIKE ? OR lower(push_name) LIKE ? OR lower(their_jid) LIKE ?
			LIMIT 20
		`, "%"+query+"%", "%"+query+"%", "%"+query+"%")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var contacts []Contact
		for rows.Next() {
			var c Contact
			if err := rows.Scan(&c.JID, &c.Name, &c.PushName); err == nil {
				contacts = append(contacts, c)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contacts)
	})

	http.HandleFunc("/api/check-number", func(w http.ResponseWriter, r *http.Request) {
		phone := r.URL.Query().Get("phone")
		if phone == "" {
			http.Error(w, "Phone number parameter is required", http.StatusBadRequest)
			return
		}

		cleanPhone := strings.TrimPrefix(phone, "+")
		resp, err := client.IsOnWhatsApp(context.Background(), []string{"+" + cleanPhone})
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		isOnWhatsApp := len(resp) > 0 && resp[0].IsIn
		jidStr := ""
		if isOnWhatsApp {
			jidStr = resp[0].JID.String()
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":        true,
			"is_on_whatsapp": isOnWhatsApp,
			"jid":            jidStr,
			"phone":          cleanPhone,
		})
	})

	http.HandleFunc("/api/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req DownloadMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MessageID == "" || req.ChatJID == "" {
			http.Error(w, "Invalid request parameters", http.StatusBadRequest)
			return
		}

		success, mediaType, filename, path, err := downloadMedia(client, messageStore, req.MessageID, req.ChatJID)
		w.Header().Set("Content-Type", "application/json")

		if !success || err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DownloadMediaResponse{Success: false, Message: fmt.Sprintf("Failed: %v", err)})
			return
		}

		json.NewEncoder(w).Encode(DownloadMediaResponse{
			Success:  true,
			Message:  fmt.Sprintf("Successfully downloaded %s", mediaType),
			Filename: filename,
			Path:     path,
		})
	})
	
	// Handler to list recent chats
	http.HandleFunc("/api/chats/recent", func(w http.ResponseWriter, r *http.Request) {
		chats, err := messageStore.GetChats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chats)
	})

	// Handler to get message history for a specific chat JID
	http.HandleFunc("/api/chat/history", func(w http.ResponseWriter, r *http.Request) {
		chatJID := r.URL.Query().Get("jid")
		if chatJID == "" {
			http.Error(w, "Chat JID parameter is required", http.StatusBadRequest)
			return
		}

		messages, err := messageStore.GetMessages(chatJID, 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting REST API server on %s...\n", serverAddr)
	go func() {
		if err := http.ListenAndServe(serverAddr, nil); err != nil {
			fmt.Printf("REST API server error: %v\n", err)
		}
	}()
}