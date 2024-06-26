package pgstore

import (
	"context"

	openapi_types "github.com/discord-gophers/goapi-gen/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateTripParams struct {
	Destination string
	OwnerEmail  string
	OwnerName   string
	StartsAt    pgtype.Timestamp
	EndsAt      pgtype.Timestamp

	Participants []openapi_types.Email
}

func (q *Queries) CreateTrip(ctx context.Context, pool *pgxpool.Pool, params CreateTripParams) (uuid.UUID, error) {
	var id uuid.UUID
	tx, err := pool.Begin(ctx)
	if err != nil {
		return id, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := q.WithTx(tx)

	tripID, err := qtx.InsertTrip(
		ctx,
		InsertTripParams{
			Destination: params.Destination,
			OwnerEmail:  params.OwnerEmail,
			OwnerName:   params.OwnerName,
			StartsAt:    params.StartsAt,
			EndsAt:      params.EndsAt,
		},
	)
	if err != nil {
		return id, err
	}

	participants := make([]InviteParticipantsToTripParams, len(params.Participants))
	for i, p := range params.Participants {
		participants[i] = InviteParticipantsToTripParams{
			TripID: tripID,
			Email:  string(p),
		}
	}

	if _, err := qtx.InviteParticipantsToTrip(ctx, participants); err != nil {
		return id, err
	}

	return tripID, tx.Commit(ctx)
}
