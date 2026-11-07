package pulse

import (
	"fmt"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/desktop"
	"github.com/hardhacker/hylo/internal/mate"
)

// Where a message from the bot would find the user.
const (
	reachUnknown  = ""         // no desktop reports, no sign of the phone
	reachComputer = "computer" // at the computer
	reachPhone    = "phone"    // away from it, but wrote from a connected app lately
	reachPush     = "push"     // away; a message would buzz their phone
	reachNone     = "none"     // away; a message waits on the desktop
)

// phoneWithin is how recent a message from a connected app shows the phone in hand.
const phoneWithin = 30 * time.Minute

// reachOdds scales the odds by where the user is: rounds are worth more when
// someone is there to hear them, and a phone still hears.
var reachOdds = map[string]float64{reachPush: 0.7, reachNone: 0.5}

type reach struct {
	Where string
	App   string    // the connected app they last wrote from
	AppAt time.Time // when
	Push  []string  // connected apps that can reach their phone
}

func (p *Pulse) reach(mateID string, desk desktop.State, now time.Time) reach {
	r := reach{}
	r.App, r.AppAt, _ = p.store.LastAppMessage(mateID)
	if p.pushable != nil {
		r.Push = p.pushable(mateID)
	}
	switch {
	case desk.Present:
		r.Where = reachComputer
	case r.App != "" && now.Sub(r.AppAt) <= phoneWithin:
		r.Where = reachPhone
	case !desk.Known:
		r.Where = reachUnknown
	case len(r.Push) > 0:
		r.Where = reachPush
	default:
		r.Where = reachNone
	}
	return r
}

// onPhone reports whether a message would land on the user's phone.
func (r reach) onPhone() bool { return r.Where == reachPhone || r.Where == reachPush }

func (r reach) line(now time.Time) string {
	switch r.Where {
	case reachPhone:
		return fmt.Sprintf("- On their phone: wrote to you from %s %s. A message reaches them there.", appName(r.App), ago(now.Sub(r.AppAt)))
	case reachPush:
		return "- Away from the computer; a message would buzz their phone through " + appNames(r.Push) + "."
	case reachNone:
		return "- Away from the computer, and no connected app reaches their phone: a message waits on the desktop."
	case reachUnknown:
		if len(r.Push) > 0 {
			return "- A message also reaches their phone through " + appNames(r.Push) + "."
		}
	}
	return ""
}

func appName(kind string) string {
	switch kind {
	case mate.ChannelTelegram:
		return "Telegram"
	case mate.ChannelDiscord:
		return "Discord"
	case mate.ChannelWechat:
		return "WeChat"
	}
	return kind
}

func appNames(kinds []string) string {
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = appName(k)
	}
	return strings.Join(names, " and ")
}
