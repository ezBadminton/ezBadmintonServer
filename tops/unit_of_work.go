package tops

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"sync"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/subscriptions"
)

// UnitOfWorkManager mutually exludes "units of work" from each other
// by using a mutex object from the sync library.
// It also assigns a randomly generated ID to each unit of work.
// This server software upates its clients using SSE realtime messages,
// and this ID is used to group SSE messages together. Upon receiving
// an update, the clients know wheter more upates are about to
// arrive that are a part of the same unit.
// After the last update of a unit, the server sends a meta-message,
// signalling the end of the unit of work.
// Only then the client has to react to the changes (e.g. updating the UI).
type UnitOfWorkManager struct {
	mu sync.RWMutex

	messages chan unitOfWorkMessage

	app                core.App
	unitOfWorkId       string
	unitOfWorkCounters map[string]int
}

type unitOfWorkMessage struct {
	startUnitOfWork bool
	addItem         bool
	removeItem      bool
	unitOfWorkId    string
}

func newUnitOfWorkManager(app core.App) *UnitOfWorkManager {
	m := &UnitOfWorkManager{
		app:                app,
		unitOfWorkCounters: make(map[string]int),
		messages:           make(chan unitOfWorkMessage, 1),
	}

	app.OnRecordEnrich().BindFunc(m.enrichUnitOfWork)

	app.OnRecordCreate().BindFunc(m.onCrud)
	app.OnRecordUpdate().BindFunc(m.onCrud)
	app.OnRecordDelete().BindFunc(m.onCrud)

	app.OnRecordAfterCreateError().BindFunc(m.onCrudError)
	app.OnRecordAfterUpdateError().BindFunc(m.onCrudError)
	app.OnRecordAfterDeleteError().BindFunc(m.onCrudError)

	app.OnRealtimeMessageSend().BindFunc(m.onRealtimeMessage)

	go m.workItemRoutine()

	return m
}

func (m *UnitOfWorkManager) startUnitOfWork() {
	m.mu.Lock()
	m.unitOfWorkId = newUnitOfWorkId()
	unitOfWorkMessage := unitOfWorkMessage{
		startUnitOfWork: true,
		unitOfWorkId:    m.unitOfWorkId,
	}
	m.messages <- unitOfWorkMessage
}

func (m *UnitOfWorkManager) endUnitOfWork() {
	defer m.mu.Unlock()
	m.unitOfWorkId = ""
}

func (m *UnitOfWorkManager) workItemRoutine() {
	for workItem := range m.messages {
		if workItem.unitOfWorkId == "" {
			continue
		}
		unitOfWorkId := workItem.unitOfWorkId
		if workItem.startUnitOfWork {
			m.unitOfWorkCounters[unitOfWorkId] = 0
		} else if workItem.addItem {
			m.unitOfWorkCounters[unitOfWorkId] += 1
		} else if workItem.removeItem {
			m.unitOfWorkCounters[unitOfWorkId] -= 1
			if m.unitOfWorkCounters[unitOfWorkId] == 0 {
				delete(m.unitOfWorkCounters, unitOfWorkId)
				go realtimeEndUnitOfWork(m.app, unitOfWorkId)
			}
		}
	}
}

func (m *UnitOfWorkManager) addWorkItem() string {
	if m.unitOfWorkId == "" {
		return ""
	}
	workItemMessage := unitOfWorkMessage{
		addItem:      true,
		unitOfWorkId: m.unitOfWorkId,
	}
	m.messages <- workItemMessage
	return m.unitOfWorkId
}

func (m *UnitOfWorkManager) removeWorkItem(unitOfWorkId string) {
	if unitOfWorkId == "" {
		return
	}
	workItemMessage := unitOfWorkMessage{
		removeItem:   true,
		unitOfWorkId: unitOfWorkId,
	}
	m.messages <- workItemMessage
}

func (m *UnitOfWorkManager) enrichUnitOfWork(e *core.RecordEnrichEvent) error {
	if e.RequestInfo.Context == core.RequestInfoContextRealtime {
		m.addUnitOfWorkToRecord(e.Record)
	}
	return e.Next()
}

func (m *UnitOfWorkManager) addUnitOfWorkToRecord(record *core.Record) {
	isUnitOfWorkActive := m.unitOfWorkId != ""
	if !isUnitOfWorkActive {
		return
	}
	record.Set("unitOfWork", m.unitOfWorkId)
	record.WithCustomData(true)
}

func (m *UnitOfWorkManager) onCrud(e *core.RecordEvent) error {
	if m.unitOfWorkId != "" {
		e.Record.Set("unitOfWork", m.unitOfWorkId)
		e.Record.WithCustomData(true)
		m.addWorkItem()
	}
	return e.Next()
}

func (m *UnitOfWorkManager) onCrudError(e *core.RecordErrorEvent) error {
	unitOfWorkId := e.Record.GetString("unitOfWork")
	if unitOfWorkId != "" {
		m.removeWorkItem(unitOfWorkId)
	}
	return e.Next()
}

func (m *UnitOfWorkManager) onRealtimeMessage(e *core.RealtimeMessageEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	go m.registerOutgoingUnitOfWorkMessage(e.Message.Data)
	return nil
}

// registerOutgoingUnitOfWorkMessage has to unmarshal all messages to check for
// records with the unitOfWork field set. There is no other way to do this at the
// moment. The OnRecordEnrich hook is "too early" as it is not synchronous with
// the actual message sending, but we do need the exact time of sending.
func (m *UnitOfWorkManager) registerOutgoingUnitOfWorkMessage(message []byte) {
	data := struct {
		Record unitOfWorkData `json:"record"`
	}{}
	json.Unmarshal(message, &data)
	unitOfWorkId := data.Record.UnitOfWorkId
	if unitOfWorkId != "" {
		m.removeWorkItem(unitOfWorkId)
	}
}

func newUnitOfWorkId() string {
	b := make([]byte, 12)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

type unitOfWorkData struct {
	UnitOfWorkId string `json:"unitOfWork"`
}

// realtimeEndUnitOfWork sends a realtime message to all subscribers of "unitOfWork".
// It signals to the subscibers that the current unit of work has been completed.
func realtimeEndUnitOfWork(app core.App, unitOfWorkId string) error {
	data := unitOfWorkData{unitOfWorkId}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	message := subscriptions.Message{
		Name: "unitOfWorkEnd",
		Data: jsonData,
	}

	clients := app.SubscriptionsBroker().Clients()
	for _, client := range clients {
		if client.IsDiscarded() || !client.HasSubscription("unitOfWorkEnd") {
			continue
		}
		auth, ok := client.Get("auth").(*core.Record)
		if !ok || auth == nil {
			continue
		}
		client.Send(message)
	}

	return nil
}
