package mailpit

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phenpessoa/rocketseat-journey/internal/pgstore"
	"github.com/wneessen/go-mail"
)

type Store interface {
	GetTrip(ctx context.Context, id uuid.UUID) (pgstore.Trip, error)
	GetParticipant(ctx context.Context, participantID uuid.UUID) (pgstore.Participant, error)
	GetParticipants(ctx context.Context, tripID uuid.UUID) ([]pgstore.Participant, error)
}

type Mailpit struct {
	store Store
}

func New(pool *pgxpool.Pool) Mailpit {
	return Mailpit{store: pgstore.New(pool)}
}

func (m Mailpit) SendTripConfirmationEmail(tripID uuid.UUID) error {
	ctx := context.Background()
	trip, err := m.store.GetTrip(ctx, tripID)
	if err != nil {
		return err
	}

	msg := mail.NewMsg()
	if err := msg.From("mailpit@journey.com"); err != nil {
		return err
	}

	if err := msg.To(trip.OwnerEmail); err != nil {
		return err
	}

	msg.Subject("Confirme sua viagem")
	msg.SetBodyString(mail.TypeTextPlain, "Você deve confirmar sua viagem")

	c, err := mail.NewClient("localhost", mail.WithTLSPortPolicy(mail.NoTLS), mail.WithPort(1025))
	if err != nil {
		return err
	}

	if err := c.DialAndSend(msg); err != nil {
		return err
	}

	return nil
}

func (m Mailpit) SendTripConfirmedEmails(tripID uuid.UUID) error {
	participants, err := m.store.GetParticipants(context.Background(), tripID)
	if err != nil {
		return err
	}

	c, err := mail.NewClient("localhost", mail.WithTLSPortPolicy(mail.NoTLS), mail.WithPort(1025))
	if err != nil {
		return err
	}

	for _, p := range participants {
		msg := mail.NewMsg()
		if err := msg.From("mailpit@journey.com"); err != nil {
			return err
		}

		if err := msg.To(p.Email); err != nil {
			return err
		}

		msg.Subject("Confirme sua viagem")
		msg.SetBodyString(mail.TypeTextPlain, "Você deve confirmar sua viagem")

		if err := c.DialAndSend(msg); err != nil {
			return err
		}
	}

	return nil
}

func (m Mailpit) SendTripConfirmedEmail(tripID, participantID uuid.UUID) error {
	ctx := context.Background()
	participant, err := m.store.GetParticipant(ctx, participantID)
	if err != nil {
		return err
	}

	msg := mail.NewMsg()
	if err := msg.From("mailpit@journey.com"); err != nil {
		return err
	}

	if err := msg.To(participant.Email); err != nil {
		return err
	}

	msg.Subject("Confirme sua viagem")
	msg.SetBodyString(mail.TypeTextPlain, "Você deve confirmar sua viagem")

	c, err := mail.NewClient("localhost", mail.WithTLSPortPolicy(mail.NoTLS), mail.WithPort(1025))
	if err != nil {
		return err
	}

	if err := c.DialAndSend(msg); err != nil {
		return err
	}

	return nil
}
