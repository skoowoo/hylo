package handler

import (
	"net/http"
)

// StatusResponse is the JSON body returned by POST /api/status.
type StatusResponse struct {
	Notes         int    `json:"notes"`
	Indexed       uint64 `json:"indexed"`
	KnowledgeNotes int   `json:"knowledge_notes"`
	ShortDays     int    `json:"short_days"`
}

// NoteCounter is satisfied by storage.Vault.
type NoteCounter interface {
	CountNotes() (int, error)
	CountIndexedNotes() (int, error)
	CountKnowledgeNotes() (int, error)
	CountShortDays() (int, error)
}

// StatusHandler handles POST /api/status.
type StatusHandler struct {
	notes NoteCounter
}

// NewStatus returns a StatusHandler.
func NewStatus(notes NoteCounter) *StatusHandler {
	return &StatusHandler{notes: notes}
}

// Status handles POST /api/status.
func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	notes, err := h.notes.CountNotes()
	if err != nil {
		http.Error(w, "status: count notes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	indexed, err := h.notes.CountIndexedNotes()
	if err != nil {
		http.Error(w, "status: count indexed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	knowledgeNotes, err := h.notes.CountKnowledgeNotes()
	if err != nil {
		http.Error(w, "status: count knowledge notes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	shortDays, err := h.notes.CountShortDays()
	if err != nil {
		http.Error(w, "status: count short days: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, StatusResponse{
		Notes:          notes,
		Indexed:        uint64(indexed),
		KnowledgeNotes: knowledgeNotes,
		ShortDays:      shortDays,
	})
}
