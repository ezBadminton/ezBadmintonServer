package tops

import (
	"encoding/json"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
)

type realtimeNotifier interface {
	AddRealtimeNotification(
		app core.App,
		subscription string,
		action string,
		topsRecord TopsRecord,
		priority int,
	)
}

type realtimeNotificationList struct {
	notifications []realtimeNotification
}

func (l *realtimeNotificationList) AddRealtimeNotification(
	app core.App,
	subscription string,
	action string,
	topsRecord TopsRecord,
	priority int,
) {
	trigger := func() {
		realtimeNotify(app, subscription, action, topsRecord)
	}
	notification := realtimeNotification{
		priority: priority,
		trigger:  trigger,
	}
	l.notifications = append(l.notifications, notification)
}

func (l *realtimeNotificationList) TriggerRealtimeNotifications() {
	slices.SortStableFunc(l.notifications, func(a, b realtimeNotification) int {
		if a.priority < b.priority {
			return -1
		}
		if a.priority > b.priority {
			return 1
		}
		return 0
	})
	for _, n := range l.notifications {
		n.trigger()
	}
}

type realtimeNotification struct {
	priority int
	trigger  func()
}

type realtimeMessage struct {
	Record map[string]any `json:"record"`
	Action string         `json:"action"`
}

func realtimeNotify(app core.App, subscription string, action string, topsRecord TopsRecord) error {
	subscription += "/*"
	actionData := realtimeMessage{
		Record: topsRecord.ToMap(),
		Action: action,
	}

	jsonData, err := json.Marshal(actionData)
	if err != nil {
		return err
	}

	message := subscriptions.Message{
		Name: subscription,
		Data: jsonData,
	}

	clients := app.SubscriptionsBroker().Clients()
	for _, client := range clients {
		if !client.HasSubscription(subscription) {
			continue
		}
		auth, ok := client.Get("auth").(*core.Record)
		if !ok || auth.Collection().Name != CName[TournamentOrganizer]() {
			continue
		}
		client.Send(message)
	}

	return nil
}
