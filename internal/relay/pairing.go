package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	pairInviteTTL      = 2 * time.Minute
	maxActivePairCodes = 32
)

// DeviceRecord persists an authorized mobile device on the Relay. Only the
// SHA-256 digest of the device token is stored on disk.
type DeviceRecord struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id,omitempty"`
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	CreatedAt time.Time `json:"created_at"`
}

// DeviceInfo is the safe public view of a paired device.
type DeviceInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type pairInvite struct {
	serverID  string
	expiresAt time.Time
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (r *Relay) pairCreate(w http.ResponseWriter, req *http.Request) {
	id, _, ok := r.connectorAuth(req, false)
	if !ok {
		writeRelayError(w, http.StatusUnauthorized, "relay_unauthorized", "invalid connector credentials")
		return
	}
	now := time.Now()
	expiresAt := now.Add(pairInviteTTL)
	code := "pair_" + randomID()

	r.mu.Lock()
	for k, inv := range r.pairings {
		if now.After(inv.expiresAt) {
			delete(r.pairings, k)
		}
	}
	if len(r.pairings) >= maxActivePairCodes {
		r.mu.Unlock()
		writeRelayError(w, http.StatusTooManyRequests, "relay_busy", "too many pending pairing codes")
		return
	}
	r.pairings[code] = &pairInvite{serverID: id, expiresAt: expiresAt}
	r.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":       code,
		"server_id":  id,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (r *Relay) pairRedeem(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ServerID   string `json:"server_id"`
		Code       string `json:"code"`
		DeviceName string `json:"device_name"`
		ClientID   string `json:"client_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 8*1024)).Decode(&body); err != nil {
		writeRelayError(w, http.StatusBadRequest, "invalid_request", "invalid JSON payload")
		return
	}
	serverID := strings.TrimSpace(body.ServerID)
	code := strings.TrimSpace(body.Code)
	name := strings.TrimSpace(body.DeviceName)
	clientID := strings.TrimSpace(body.ClientID)
	if name == "" {
		name = "iPhone"
	}
	if len(name) > 64 {
		name = name[:64]
	}
	if len(clientID) > 64 {
		clientID = clientID[:64]
	}
	if serverID == "" || code == "" {
		writeRelayError(w, http.StatusBadRequest, "invalid_request", "server_id and code are required")
		return
	}

	now := time.Now()
	deviceID := "dev_" + randomID()[:12]
	deviceToken := "rlyd_" + randomID()
	rec := DeviceRecord{
		ID:        deviceID,
		ClientID:  clientID,
		Name:      name,
		TokenHash: tokenHash(deviceToken),
		CreatedAt: now.UTC(),
	}

	r.mu.Lock()
	inv, ok := r.pairings[code]
	if !ok || inv.serverID != serverID {
		r.mu.Unlock()
		writeRelayError(w, http.StatusNotFound, "pair_invalid", "pairing code is invalid or already used")
		return
	}
	// Atomically consume the one-time invite before releasing the lock.
	delete(r.pairings, code)
	if now.After(inv.expiresAt) {
		r.mu.Unlock()
		writeRelayError(w, http.StatusGone, "pair_expired", "pairing code has expired")
		return
	}
	cred, exists := r.servers[serverID]
	if !exists {
		r.mu.Unlock()
		writeRelayError(w, http.StatusNotFound, "server_not_found", "server not found")
		return
	}
	oldDevices := cred.Devices
	var replacedIDs []string
	updated := make([]DeviceRecord, 0, len(cred.Devices)+1)
	for _, d := range cred.Devices {
		if clientID != "" && d.ClientID == clientID {
			replacedIDs = append(replacedIDs, d.ID)
			continue
		}
		updated = append(updated, d)
	}
	updated = append(updated, rec)
	cred.Devices = updated
	r.servers[serverID] = cred
	if err := r.persistLocked(); err != nil {
		cred.Devices = oldDevices
		r.servers[serverID] = cred
		r.mu.Unlock()
		writeRelayError(w, http.StatusInternalServerError, "relay_error", "failed to persist device")
		return
	}
	r.mu.Unlock()

	for _, oldID := range replacedIDs {
		r.cancelDeviceRequests(oldID)
	}
	r.log.Info("device paired", "server_id", serverID, "device_id", deviceID, "name", name, "replaced", len(replacedIDs))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"device_id":    deviceID,
		"device_token": deviceToken,
		"server_id":    serverID,
	})
}

func (r *Relay) listDevices(w http.ResponseWriter, req *http.Request) {
	id, cred, ok := r.connectorAuth(req, false)
	if !ok {
		writeRelayError(w, http.StatusUnauthorized, "relay_unauthorized", "invalid connector credentials")
		return
	}
	out := make([]DeviceInfo, 0, len(cred.Devices))
	for _, d := range cred.Devices {
		out = append(out, DeviceInfo{
			ID:        d.ID,
			Name:      d.Name,
			CreatedAt: d.CreatedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"server_id": id,
		"devices":   out,
	})
}

func (r *Relay) revokeDevice(w http.ResponseWriter, req *http.Request) {
	id, _, ok := r.connectorAuth(req, false)
	if !ok {
		writeRelayError(w, http.StatusUnauthorized, "relay_unauthorized", "invalid connector credentials")
		return
	}
	deviceID := strings.TrimSpace(req.PathValue("device_id"))
	if deviceID == "" {
		writeRelayError(w, http.StatusBadRequest, "invalid_request", "missing device_id")
		return
	}

	r.mu.Lock()
	cred, exists := r.servers[id]
	if !exists {
		r.mu.Unlock()
		writeRelayError(w, http.StatusNotFound, "server_not_found", "server not found")
		return
	}
	idx := -1
	for i, d := range cred.Devices {
		if d.ID == deviceID {
			idx = i
			break
		}
	}
	if idx < 0 {
		r.mu.Unlock()
		writeRelayError(w, http.StatusNotFound, "device_not_found", "device not found")
		return
	}
	updated := make([]DeviceRecord, 0, len(cred.Devices)-1)
	updated = append(updated, cred.Devices[:idx]...)
	updated = append(updated, cred.Devices[idx+1:]...)
	oldDevices := cred.Devices
	cred.Devices = updated
	r.servers[id] = cred
	if err := r.persistLocked(); err != nil {
		cred.Devices = oldDevices
		r.servers[id] = cred
		r.mu.Unlock()
		writeRelayError(w, http.StatusInternalServerError, "relay_error", "failed to persist revocation")
		return
	}
	r.mu.Unlock()

	r.cancelDeviceRequests(deviceID)
	r.log.Info("device revoked", "server_id", id, "device_id", deviceID)
	w.WriteHeader(http.StatusNoContent)
}

// revokeSelf lets a paired device revoke its own credential on disconnect.
func (r *Relay) revokeSelf(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("server_id")
	deviceID, status, code, msg := r.checkAccess(req, id)
	if status != 0 {
		writeRelayError(w, status, code, msg)
		return
	}
	if deviceID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	r.mu.Lock()
	cred, exists := r.servers[id]
	if !exists {
		r.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	updated := make([]DeviceRecord, 0, len(cred.Devices))
	removed := false
	for _, d := range cred.Devices {
		if d.ID == deviceID {
			removed = true
			continue
		}
		updated = append(updated, d)
	}
	if removed {
		oldDevices := cred.Devices
		cred.Devices = updated
		r.servers[id] = cred
		if err := r.persistLocked(); err != nil {
			cred.Devices = oldDevices
			r.servers[id] = cred
			r.mu.Unlock()
			writeRelayError(w, http.StatusInternalServerError, "relay_error", "failed to persist revocation")
			return
		}
	}
	r.mu.Unlock()

	if removed {
		r.cancelDeviceRequests(deviceID)
		r.log.Info("device disconnected and revoked", "server_id", id, "device_id", deviceID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Relay) trackDeviceRequest(deviceID string, parent context.Context) (context.Context, func()) {
	if deviceID == "" {
		return parent, func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	reqID := randomID()[:12]

	r.mu.Lock()
	if r.activeDevices[deviceID] == nil {
		r.activeDevices[deviceID] = make(map[string]context.CancelFunc)
	}
	r.activeDevices[deviceID][reqID] = cancel
	r.mu.Unlock()

	return ctx, func() {
		cancel()
		r.mu.Lock()
		if m := r.activeDevices[deviceID]; m != nil {
			delete(m, reqID)
			if len(m) == 0 {
				delete(r.activeDevices, deviceID)
			}
		}
		r.mu.Unlock()
	}
}

func (r *Relay) cancelDeviceRequests(deviceID string) {
	r.mu.Lock()
	cancels := r.activeDevices[deviceID]
	delete(r.activeDevices, deviceID)
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
