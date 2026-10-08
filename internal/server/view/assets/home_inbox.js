// Matches internal/inbox.defaultListLimit — the server-side page size used
// when a request omits ?limit=. Kept in sync manually since the client
// needs to know a full page was returned to decide whether more exist.
var INBOX_PAGE_SIZE = 50;

function homeInboxMixin() {
  return {
    // ── Inbox ────────────────────────────────────────────────────────────
    inboxMessages: [],
    inboxLoading: true,
    inboxLoadingMore: false,
    inboxHasMore: true,
    inboxFilter: 'all',
    inboxSelected: null,
    inboxSheetOpen: false,
    unreadCount: 0,
    initInbox() {
      // No longer pushed onto the shared ESC stack — the inbox detail is a
      // docked pane (content_pane.html's inbox-detail-panel), not a floating
      // overlay, so Esc shouldn't dismiss it.
      this.$watch('inboxSheetOpen', (val) => {
        if (!val) return;
        // The note editor and the inbox detail share one pane (see
        // content_pane.html's inbox-detail-panel) — always one or the other.
        if (this.contentPaneOpen) this.contentPaneOpen = false;
        var overlayEl = document.querySelector('.content-pane');
        if (overlayEl) {
          overlayEl.classList.add('content-pane-is-opening');
          setTimeout(function () { overlayEl.classList.remove('content-pane-is-opening'); }, 320);
        }
      });
      // The unread badge in the sidebar must stay correct regardless of
      // which section is currently open, so both the initial count and the
      // live SSE subscription start unconditionally here.
      this.refreshUnreadCount();
      this.initInboxStream();
    },
    // ── Inbox: message list + unread badge + read-only sheet ───────────────
    initInboxStream() {
      // Subscribes to /api/inbox/notifications (internal/inbox.Bus) so a
      // message created while home is open updates the badge/list live
      // instead of waiting for a manual refresh. Runs regardless of which
      // sidebar section is active since the unread badge is always visible.
      if (typeof EventSource === 'undefined') return;
      var es = new EventSource('/api/inbox/notifications');
      es.addEventListener('message', (e) => {
        var msg;
        try { msg = JSON.parse(e.data); } catch (err) { return; }
        if (this.inboxMessages.some((m) => m.id === msg.id)) return;
        this.inboxMessages.unshift(msg);
        if (!msg.isRead) this.unreadCount++;
      });
    },
    visibleInboxMessages() {
      if (this.inboxFilter === 'all') return this.inboxMessages;
      return this.inboxMessages.filter((m) => !m.isRead);
    },
    setInboxFilter(f) {
      this.inboxFilter = f;
      this.fillInboxList();
    },
    loadInbox() {
      this.inboxLoading = true;
      this.inboxHasMore = true;
      return fetch('/api/inbox').then(function (r) { return r.json(); }).then((data) => {
        this.inboxMessages = data.messages || [];
        this.inboxHasMore = this.inboxMessages.length >= INBOX_PAGE_SIZE;
      }).catch(function () { }).then(() => {
        this.inboxLoading = false;
        this.refreshUnreadCount();
        this.fillInboxList();
      });
    },
    loadMoreInbox() {
      if (this.inboxLoadingMore || !this.inboxHasMore) return Promise.resolve();
      this.inboxLoadingMore = true;
      return fetch('/api/inbox?offset=' + this.inboxMessages.length).then(function (r) { return r.json(); }).then((data) => {
        var page = data.messages || [];
        var seen = new Set(this.inboxMessages.map((m) => m.id));
        page.forEach((m) => { if (!seen.has(m.id)) this.inboxMessages.push(m); });
        this.inboxHasMore = page.length >= INBOX_PAGE_SIZE;
      }).catch(function () { }).then(() => {
        this.inboxLoadingMore = false;
      });
    },
    onInboxListScroll(e) {
      var el = e.target;
      if (el.scrollTop + el.clientHeight >= el.scrollHeight - 120) this.loadMoreInbox();
    },
    // A short (e.g. unread-filtered) list never fires scroll, so page in
    // until it overflows or the server runs dry.
    async fillInboxList() {
      await this.$nextTick();
      var el = document.getElementById('home-inbox-list');
      while (el && this.inboxHasMore && !this.inboxLoadingMore && el.scrollHeight <= el.clientHeight + 4) {
        await this.loadMoreInbox();
        await this.$nextTick();
      }
    },
    refreshUnreadCount() {
      return fetch('/api/inbox/unread-count').then(function (r) { return r.json(); }).then((data) => {
        this.unreadCount = data.count || 0;
      }).catch(function () { });
    },
    selectInboxMessage(m) {
      this.inboxSelected = m;
      this.inboxSheetOpen = true;
      if (!m.isRead) this.markInboxRead(m);
    },
    closeInboxSheet() {
      this.inboxSheetOpen = false;
    },
    async markInboxRead(m) {
      m.isRead = true;
      this.unreadCount = Math.max(0, this.unreadCount - 1);
      try { await fetch('/api/inbox/' + m.id + '/read', { method: 'POST' }); } catch (e) { /* ignore */ }
    },
    async markAllInboxRead() {
      this.inboxMessages.forEach((m) => { m.isRead = true; });
      this.unreadCount = 0;
      try { await fetch('/api/inbox/read-all', { method: 'POST' }); } catch (e) { /* ignore */ }
    },
    // The card shows one ellipsized line; the full body would still be laid
    // out on every reattach, which grows with message length.
    inboxSnippet(body) {
      return (body || '').slice(0, 200).replace(/\s+/g, ' ').trim();
    },
    relTime(iso) {
      const d = new Date(iso);
      const diffSec = Math.floor((Date.now() - d.getTime()) / 1000);
      if (diffSec < 60) return 'now';
      if (diffSec < 3600) return Math.floor(diffSec / 60) + 'm';
      if (diffSec < 86400) return Math.floor(diffSec / 3600) + 'h';
      return Math.floor(diffSec / 86400) + 'd';
    },
    fullTime(iso) {
      try { return new Date(iso).toLocaleString(); } catch (e) { return iso; }
    }
  };
}
