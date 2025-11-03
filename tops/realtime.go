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
	unitOfWork := tops.addWorkItem()
	trigger := func() {
		realtimeNotify(app, subscription, action, topsRecord, unitOfWork)
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

func realtimeNotify(app core.App, subscription string, action string, topsRecord TopsRecord, unitOfWork string) error {
	// The '/*' suffix is added because the pocketbase client SDK subscribes to 'collection_name/*'
	// when using its subscribe function. With this, this function can be passed just the collection name.
	subscription += "/*"

	recordMap := topsRecord.ToMap()
	if len(unitOfWork) > 0 {
		recordMap["unitOfWork"] = unitOfWork
		defer tops.removeWorkItem(unitOfWork)
	}

	actionData := realtimeMessage{
		Record: recordMap,
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
	pingedAuths := []string{CName[TournamentOrganizer](), CName[InfoscreenUser]()}
	for _, client := range clients {
		if client.IsDiscarded() || !client.HasSubscription(subscription) {
			continue
		}
		auth, ok := client.Get("auth").(*core.Record)
		if !ok || auth == nil || !slices.Contains(pingedAuths, auth.Collection().Name) {
			continue
		}
		client.Send(message)
	}

	return nil
}

// To prevent the SSE connections from being cleaned-up after being idle
// the organizers are pinged
func PingOrganizerClients(app core.App) error {
	message := subscriptions.Message{
		Name: "ping",
	}

	clients := app.SubscriptionsBroker().Clients()
	pingedAuths := []string{CName[TournamentOrganizer](), CName[InfoscreenUser]()}
	for _, client := range clients {
		if client.IsDiscarded() || !client.HasSubscription("ping") {
			continue
		}
		auth, ok := client.Get("auth").(*core.Record)
		if !ok || auth == nil || !slices.Contains(pingedAuths, auth.Collection().Name) {
			continue
		}
		client.Send(message)
	}

	return nil
}
