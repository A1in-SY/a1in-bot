package event

import (
	qqbot "qqbot/api"
)

type EventHandler interface {
	Match(*qqbot.NotifyEvent) bool
	Handle(*qqbot.NotifyEvent)
}
