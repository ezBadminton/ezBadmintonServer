package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

var (
	ErrInvalidDiscount      = errors.New("the discount percent has to be between 0 and 100")
	ErrInvalidPaymentAmount = errors.New("the payment amount does not equal the owed amount.")
)

type StartingFeeManager struct {
	app core.App

	onPayment *hook.Hook[*StartingFeePaymentEvent]
}

func newStartingFeeManager(app core.App) *StartingFeeManager {
	return &StartingFeeManager{
		app:       app,
		onPayment: &hook.Hook[*StartingFeePaymentEvent]{},
	}
}

func (m *StartingFeeManager) setStartingFees(competitions []*Competition, startingFee int) error {
	return m.app.RunInTransaction(func(txApp core.App) error {
		for _, competition := range competitions {
			competition := Clone(competition)
			competition.SetStartingFee(startingFee)
			if err := txApp.Save(competition); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *StartingFeeManager) payFee(player *Player, amount, discountPercent int) error {
	if discountPercent < 0 || discountPercent > 100 {
		return ErrInvalidDiscount
	}

	event := newStartingFeePaymentEvent(m.app, player, amount, discountPercent)
	err := m.onPayment.Trigger(event,
		m.handlePayment,
		(*StartingFeePaymentEvent).savePayment,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
}

func (m *StartingFeeManager) handlePayment(e *StartingFeePaymentEvent) error {
	feeAmount, massDiscount := m.calculatePlayerFee(e.Player, e.Registrations)
	if feeAmount != e.Amount {
		return ErrInvalidPaymentAmount
	}

	payment, err := NewProxy[StartingFeePayment](e.App)
	if err != nil {
		return err
	}
	payment.SetPlayer(e.Player)
	payment.SetAmount(e.Amount)
	payment.SetDiscountPercent(e.DiscountPercent)
	payment.SetMassDiscount(massDiscount)
	e.Payment = payment
	return e.Next()
}

// Returns the net fees owed by the player and the mass discount that was subtracted from the net amount
func (m *StartingFeeManager) calculatePlayerFee(player *Player, registeredCompetitions []*Competition) (int, int) {
	amountPaid := 0
	massDiscountUsed := 0
	paymentStore, _ := store.FindRecordStore[StartingFeePayment]()
	for _, payment := range paymentStore.RecordList {
		if payment.Player().Id == player.Id {
			amountPaid += payment.Amount()
			massDiscountUsed += payment.MassDiscount()
		}
	}

	numRegistrations := len(registeredCompetitions)
	tournamentEventStore, _ := store.FindRecordStore[TournamentEvent]()
	tournamentEvent := tournamentEventStore.RecordList[0]
	massDiscounts := tournamentEvent.StartingFeeMassDiscounts()
	applicableMassDiscount := 0
	for _, massDiscount := range massDiscounts {
		discountAmount := massDiscount.DiscountAmount()
		eligible := numRegistrations >= massDiscount.MinRegistrations()
		isMax := applicableMassDiscount < discountAmount
		if eligible && isMax {
			applicableMassDiscount = discountAmount
		}
	}

	netMassDiscount := applicableMassDiscount - massDiscountUsed

	grossFees := 0
	for _, competition := range registeredCompetitions {
		grossFees += competition.StartingFee()
	}

	netFees := grossFees - netMassDiscount - amountPaid

	return netFees, netMassDiscount
}

func (m *StartingFeeManager) updateOrCreateMassDiscount(minRegistrations, amount int) error {
	tournamentStore, err := store.FindRecordStore[TournamentEvent]()
	if err != nil {
		return err
	}
	tournament := tournamentStore.RecordList[0]
	currentMassDiscounts := tournament.StartingFeeMassDiscounts()
	var massDiscount *StartingFeeMassDiscount
	isUpdate := false
	for _, discount := range currentMassDiscounts {
		if discount.MinRegistrations() == minRegistrations {
			massDiscount = discount
			isUpdate = true
			break
		}
	}

	if massDiscount == nil {
		massDiscount, _ = NewProxy[StartingFeeMassDiscount](m.app)
		massDiscount.SetMinRegistrations(minRegistrations)
	}
	massDiscount.SetDiscountAmount(amount)

	err = m.app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(massDiscount); err != nil {
			return err
		}
		if isUpdate {
			return nil
		}
		updatedMassDiscounts := append(currentMassDiscounts, massDiscount)
		tournament.SetStartingFeeMassDiscounts(updatedMassDiscounts)
		if err := txApp.Save(tournament); err != nil {
			return err
		}
		return nil
	})

	return err
}
