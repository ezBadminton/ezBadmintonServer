package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/infoscreens"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
)

func BindInfoScreenControlHooks(app core.App) {
	url := "/api/ezbadminton/infoscreen"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.BindFunc(checkControllerToken)

		group.POST("/control", broadcastControlCommand)
		group.GET("/items", getInfoItems)

		return e.Next()
	})
}

func checkControllerToken(e *core.RequestEvent) error {
	token := ""

	authHeader := e.Request.Header["Authorization"]
	if len(authHeader) > 0 {
		token = authHeader[0]
	}

	if token == "" {
		return e.BadRequestError("header does not contain an Authorization token", nil)
	}

	infoscreenUser, err := infoscreens.Tokens.ValidateToken(token)
	if errors.Is(err, infoscreens.ErrInvalidToken) {
		return e.UnauthorizedError("invalid token", err)
	} else if err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	e.Set("infoscreen_user", infoscreenUser)

	return e.Next()
}

func broadcastControlCommand(e *core.RequestEvent) error {
	infoscreenUser := e.Get("infoscreen_user").(*InfoscreenUser)
	var body = map[string]any{}
	e.BindBody(&body)

	jsonData, err := json.Marshal(body)
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	subscription := fmt.Sprintf("infoscreencontrol:%v", infoscreenUser.Id)

	message := subscriptions.Message{
		Name: subscription,
		Data: jsonData,
	}

	clients := e.App.SubscriptionsBroker().Clients()
	for _, client := range clients {
		if client.IsDiscarded() || !client.HasSubscription(subscription) {
			continue
		}
		client.Send(message)
	}

	return e.NoContent(http.StatusOK)
}

func getInfoItems(e *core.RequestEvent) error {
	infoscreenUser := e.Get("infoscreen_user").(*InfoscreenUser)
	items := infoscreenUser.InfoItems()
	e.Response.Header().Set("Content-Type", "application/json")
	e.Response.WriteHeader(http.StatusOK)
	e.Response.Write([]byte(items))
	return nil
}
