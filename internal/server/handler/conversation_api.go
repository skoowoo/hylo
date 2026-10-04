package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hardhacker/hylo/internal/mate"
)

// ConversationAPI handles conversation CRUD endpoints.
type ConversationAPI struct {
	store *mate.Store
}

// NewConversationAPI constructs the handler.
func NewConversationAPI(store *mate.Store) *ConversationAPI {
	return &ConversationAPI{store: store}
}

// ConversationsGET handles GET /api/conversations?mateId=xxx[&type=chat|trigger|trigger_reply].
// Returns conversations of the requested type for the given mate, ordered by most recent first.
// Defaults to type=chat when the parameter is absent or empty.
func (c *ConversationAPI) ConversationsGET(w http.ResponseWriter, r *http.Request) {
	mateID := strings.TrimSpace(r.URL.Query().Get("mateId"))
	convType := strings.TrimSpace(r.URL.Query().Get("type"))
	if convType == "" {
		convType = mate.ConvTypeChat
	}
	list, err := c.store.ListConversationsByType(mateID, convType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []mate.Conversation{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"conversations": list})
}

// ConversationsPOST handles POST /api/conversations.
func (c *ConversationAPI) ConversationsPOST(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MateID string `json:"mateId"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	conv, err := c.store.EnsureNewConv(strings.TrimSpace(body.MateID), mate.ConvTypeChat, "", strings.TrimSpace(body.Title), 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"conversation": conv})
}

// ConversationGET handles GET /api/conversations/{id}.
// Returns conversation metadata + messages: all of them by default, those
// updated after ?since=<ms>, or a ?limit=N page ending before ?before=<msgId>
// (hasMore tells whether older ones remain).
func (c *ConversationAPI) ConversationGET(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	conv, err := c.store.GetConversation(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	var msgs []mate.Message
	hasMore := false
	if sinceStr := q.Get("since"); sinceStr != "" {
		if sinceMs, err2 := strconv.ParseInt(sinceStr, 10, 64); err2 == nil {
			msgs, err = c.store.ListMessagesSince(id, sinceMs)
		}
	} else if limit, err2 := strconv.Atoi(q.Get("limit")); err2 == nil && limit > 0 {
		msgs, hasMore, err = c.store.ListMessagesPage(id, q.Get("before"), min(limit, 500))
	} else {
		msgs, err = c.store.ListMessages(id)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []mate.Message{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"conversation": conv,
		"messages":     msgs,
		"hasMore":      hasMore,
	})
}

// ConversationDELETE handles DELETE /api/conversations/{id}.
func (c *ConversationAPI) ConversationDELETE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := c.store.DeleteConversation(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ConversationTitlePATCH handles PATCH /api/conversations/{id}/title.
func (c *ConversationAPI) ConversationTitlePATCH(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := c.store.UpdateConversationTitle(id, strings.TrimSpace(body.Title)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}
