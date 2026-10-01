package handler

import (
	"net/http"
	"strconv"

	"github.com/hardhacker/hylo/internal/inbox"
)

// InboxAPI serves /api/inbox endpoints.
type InboxAPI struct {
	store *inbox.Store
}

// NewInboxAPI constructs the handler.
func NewInboxAPI(store *inbox.Store) *InboxAPI {
	return &InboxAPI{store: store}
}

// InboxGET handles GET /api/inbox?unread=true&source=&limit=&offset=.
func (h *InboxAPI) InboxGET(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := inbox.ListFilter{
		Source:     q.Get("source"),
		UnreadOnly: q.Get("unread") == "true",
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		f.Offset = v
	}
	list, err := h.store.List(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []inbox.Message{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"messages": list})
}

// InboxUnreadCountGET handles GET /api/inbox/unread-count?source=.
func (h *InboxAPI) InboxUnreadCountGET(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.UnreadCount(r.URL.Query().Get("source"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"count": n})
}

// InboxReadPOST handles POST /api/inbox/{id}/read.
func (h *InboxAPI) InboxReadPOST(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.MarkRead(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// InboxReadAllPOST handles POST /api/inbox/read-all?source=.
func (h *InboxAPI) InboxReadAllPOST(w http.ResponseWriter, r *http.Request) {
	if err := h.store.MarkAllRead(r.URL.Query().Get("source")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// InboxDELETE handles DELETE /api/inbox/{id}.
func (h *InboxAPI) InboxDELETE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}
