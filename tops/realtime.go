package tops

import (
	"encoding/json"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
)

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
