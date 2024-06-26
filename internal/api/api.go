package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	openapi_types "github.com/discord-gophers/goapi-gen/types"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/phenpessoa/rocketseat-journey/internal/api/spec"
	"github.com/phenpessoa/rocketseat-journey/internal/pgstore"
	"go.uber.org/zap"
)

type Store interface {
	CreateTrip(ctx context.Context, pool *pgxpool.Pool, trip pgstore.CreateTripParams) (uuid.UUID, error)
	GetTrip(ctx context.Context, id uuid.UUID) (pgstore.Trip, error)
	UpdateTrip(ctx context.Context, params pgstore.UpdateTripParams) error

	GetParticipant(ctx context.Context, participantID uuid.UUID) (pgstore.Participant, error)
	ConfirmParticipant(ctx context.Context, participantID uuid.UUID) error
	GetParticipants(ctx context.Context, tripID uuid.UUID) ([]pgstore.Participant, error)
	InviteParticipantToTrip(ctx context.Context, params pgstore.InviteParticipantToTripParams) (uuid.UUID, error)

	CreateActivity(ctx context.Context, params pgstore.CreateActivityParams) (uuid.UUID, error)
	GetTripActivities(ctx context.Context, tripID uuid.UUID) ([]pgstore.Activity, error)

	CreateTripLink(ctx context.Context, params pgstore.CreateTripLinkParams) (uuid.UUID, error)
	GetTripLinks(ctx context.Context, tripID uuid.UUID) ([]pgstore.Link, error)
}

type Mailer interface {
	SendTripConfirmationEmail(tripID uuid.UUID) error
	SendTripConfirmedEmails(tripID uuid.UUID) error
	SendTripConfirmedEmail(tripID, participantID uuid.UUID) error
}

type API struct {
	pool   *pgxpool.Pool
	store  Store
	mailer Mailer
	logger *zap.Logger
}

func NewAPI(pool *pgxpool.Pool, mailer Mailer, logger *zap.Logger) API {
	return API{pool, pgstore.New(pool), mailer, logger}
}

// Get a trip links.
// (GET /trips/{tripId}/links)
func (api API) GetTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDLinksJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	links, err := api.store.GetTripLinks(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDLinksJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to find trip links", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDLinksJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	var output spec.GetLinksResponse

	for _, link := range links {
		output.Links = append(output.Links, spec.GetLinksResponseArray{
			ID:    link.ID.String(),
			Title: link.Title,
			URL:   link.Url,
		})
	}

	return spec.GetTripsTripIDLinksJSON200Response(output)
}

// Create a trip link.
// (POST /trips/{tripId}/links)
func (api API) PostTripsTripIDLinks(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	var body spec.PostTripsTripIDLinksJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: err.Error()})
	}

	linkID, err := api.store.CreateTripLink(r.Context(), pgstore.CreateTripLinkParams{
		TripID: id,
		Title:  body.Title,
		Url:    body.URL,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDLinksJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to find trip participants", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	return spec.PostTripsTripIDLinksJSON201Response(spec.CreateLinkResponse{LinkID: linkID.String()})
}

// Get a trip activities.
// (GET /trips/{tripId}/activities)
func (api API) GetTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	activities, err := api.store.GetTripActivities(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to find trip participants", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDActivitiesJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	var output spec.GetTripActivitiesResponse

	groupedActivites := make(map[string][]pgstore.Activity)

	for _, act := range activities {
		date := act.OccursAt.Time.Format(time.DateOnly)
		groupedActivites[date] = append(groupedActivites[date], act)
	}

	for dateStr, actsOnDate := range groupedActivites {
		var innerActs []spec.GetTripActivitiesResponseInnerArray

		for _, act := range actsOnDate {
			innerActs = append(innerActs, spec.GetTripActivitiesResponseInnerArray{
				ID:       act.ID.String(),
				OccursAt: act.OccursAt.Time,
				Title:    act.Title,
			})
		}

		date, _ := time.Parse(time.DateOnly, dateStr)
		output.Activities = append(output.Activities, spec.GetTripActivitiesResponseOuterArray{
			Date:       date,
			Activities: innerActs,
		})
	}

	return spec.GetTripsTripIDActivitiesJSON200Response(output)
}

// Create a trip activity.
// (POST /trips/{tripId}/activities)
func (api API) PostTripsTripIDActivities(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	var body spec.PostTripsTripIDActivitiesJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: err.Error()})
	}

	activityID, err := api.store.CreateActivity(r.Context(), pgstore.CreateActivityParams{
		TripID:   id,
		Title:    body.Title,
		OccursAt: pgtype.Timestamp{Time: body.OccursAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to find trip participants", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PostTripsTripIDActivitiesJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	return spec.PostTripsTripIDActivitiesJSON201Response(spec.CreateActivityResponse{ActivityID: activityID.String()})
}

// Get a trip participants.
// (GET /trips/{tripId}/participants)
func (api API) GetTripsTripIDParticipants(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDParticipantsJSON400Response(
			spec.Error{Message: "invalid uuid passed: " + err.Error()},
		)
	}

	participants, err := api.store.GetParticipants(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDParticipantsJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to find trip participants", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDParticipantsJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	var output spec.GetTripParticipantsResponse

	output.Participants = make([]spec.GetTripParticipantsResponseArray, len(participants))

	for i, p := range participants {
		var name string
		parsedEmail, err := mail.ParseAddress(p.Email)
		if err == nil {
			addr := parsedEmail.Address
			name = addr[:strings.Index(addr, "@")]
		}
		output.Participants[i] = spec.GetTripParticipantsResponseArray{
			Email:       openapi_types.Email(p.Email),
			ID:          p.ID.String(),
			IsConfirmed: p.IsConfirmed,
			Name:        &name,
		}
	}

	return spec.GetTripsTripIDParticipantsJSON200Response(output)
}

// Invite someone to the trip.
// (POST /trips/{tripId}/invites)
func (api API) PostTripsTripIDInvites(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	var body spec.PostTripsTripIDInvitesJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: err.Error()})
	}

	participantID, err := api.store.InviteParticipantToTrip(r.Context(), pgstore.InviteParticipantToTripParams{
		TripID: id,
		Email:  string(body.Email),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "trip not found"})
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "participant already invited"})
			}
		}
		api.logger.Error(
			"failed to invite participant to trip",
			zap.Error(err),
			zap.String("trip_id", tripID),
			zap.String("participant_email", string(body.Email)),
		)
		return spec.PostTripsTripIDInvitesJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	go func() {
		if err := api.mailer.SendTripConfirmedEmail(id, participantID); err != nil {
			api.logger.Error(
				"failed to send trip confirmed email",
				zap.Error(err),
				zap.String("participant_id", participantID.String()),
				zap.String("trip_id", tripID),
			)
		}
	}()

	return spec.PostTripsTripIDInvitesJSON201Response(nil)
}

// Confirms a participant on a trip.
// (PATCH /participants/{participantId}/confirm)
func (api API) PatchParticipantsParticipantIDConfirm(
	w http.ResponseWriter,
	r *http.Request,
	participantID string,
) *spec.Response {
	id, err := uuid.Parse(participantID)
	if err != nil {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "invalid uuid passed: " + err.Error()},
		)
	}

	participant, err := api.store.GetParticipant(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
				spec.Error{Message: "trip or participant not found"},
			)
		}
		api.logger.Error(
			"failed to confirm participant by id",
			zap.Error(err),
			zap.String("partcipant_id", participantID),
		)
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	if participant.IsConfirmed {
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(spec.Error{
			Message: "participant already confirmed",
		})
	}

	if err := api.store.ConfirmParticipant(r.Context(), id); err != nil {
		api.logger.Error("failed to confirm participant", zap.Error(err), zap.String("partcipant_id", participantID))
		return spec.PatchParticipantsParticipantIDConfirmJSON400Response(
			spec.Error{Message: "something went wrong, try again"},
		)
	}

	return spec.PatchParticipantsParticipantIDConfirmJSON204Response(nil)
}

// Update a trip.
// (PUT /trips/{tripId})
func (api API) PutTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	var body spec.PutTripsTripIDJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: err.Error()})
	}

	if len(body.Destination) < 4 {
		return spec.PutTripsTripIDJSON400Response(struct {
			Message string `json:"message"`
		}{Message: "destination must be ate least 4 characters long"})
	}

	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.PostTripsJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to get trip by id", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	if err := api.store.UpdateTrip(r.Context(), pgstore.UpdateTripParams{
		Destination: body.Destination,
		EndsAt:      pgtype.Timestamp{Time: body.EndsAt, Valid: true},
		StartsAt:    pgtype.Timestamp{Time: body.StartsAt, Valid: true},
		ID:          id,
		IsConfirmed: trip.IsConfirmed,
	}); err != nil {
		api.logger.Error("failed to update trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.PutTripsTripIDJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	return spec.PutTripsTripIDJSON204Response(nil)
}

// Get a trip details.
// (GET /trips/{tripId})
func (api API) GetTripsTripID(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to get trip by id", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	return spec.GetTripsTripIDJSON200Response(spec.GetTripDetailsResponse{
		Trip: spec.GetTripDetailsResponseTripObj{
			Destination: trip.Destination,
			EndsAt:      trip.EndsAt.Time,
			ID:          trip.ID.String(),
			IsConfirmed: trip.IsConfirmed,
			StartsAt:    trip.StartsAt.Time,
		},
	})
}

// Confirm a trip and send e-mail invitations.
// (GET /trips/{tripId}/confirm)
func (api API) GetTripsTripIDConfirm(w http.ResponseWriter, r *http.Request, tripID string) *spec.Response {
	id, err := uuid.Parse(tripID)
	if err != nil {
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "invalid uuid passed: " + err.Error()})
	}

	trip, err := api.store.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "trip not found"})
		}
		api.logger.Error("failed to get trip by id", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	if trip.IsConfirmed {
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "trip already confirmed"})
	}

	if err := api.store.UpdateTrip(r.Context(), pgstore.UpdateTripParams{
		Destination: trip.Destination,
		EndsAt:      trip.EndsAt,
		StartsAt:    trip.StartsAt,
		IsConfirmed: true,
		ID:          id,
	}); err != nil {
		api.logger.Error("failed to update trip", zap.Error(err), zap.String("trip_id", tripID))
		return spec.GetTripsTripIDConfirmJSON400Response(spec.Error{Message: "something went wrong, try again"})
	}

	go func() {
		if err := api.mailer.SendTripConfirmedEmails(id); err != nil {
			api.logger.Error("failed to send trip confirmed email", zap.Error(err))
		}
	}()

	return spec.GetTripsTripIDConfirmJSON204Response(nil)
}

// Create a new trip
// (POST /trips)
func (api API) PostTrips(w http.ResponseWriter, r *http.Request) *spec.Response {
	var body spec.PostTripsJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: err.Error()})
	}

	if len(body.Destination) < 4 {
		return spec.PostTripsJSON400Response(struct {
			Message string `json:"message"`
		}{Message: "destination must be ate least 4 characters long"})
	}

	id, err := api.store.CreateTrip(r.Context(), api.pool, pgstore.CreateTripParams{
		Destination:  body.Destination,
		OwnerEmail:   string(body.OwnerEmail),
		OwnerName:    body.OwnerName,
		StartsAt:     pgtype.Timestamp{Time: body.StartsAt, Valid: true},
		EndsAt:       pgtype.Timestamp{Time: body.EndsAt, Valid: true},
		Participants: body.EmailsToInvite,
	})
	if err != nil {
		return spec.PostTripsJSON400Response(spec.Error{Message: "failed to create trip, try again"})
	}

	go func() {
		if err := api.mailer.SendTripConfirmationEmail(id); err != nil {
			api.logger.Error("failed to send trip confirmation email", zap.Error(err))
		}
	}()

	return spec.PostTripsJSON201Response(spec.CreateTripResponse{TripID: id.String()})
}

func (api API) ErrorHandlerFunc(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(spec.Error{Message: err.Error()}); err != nil {
		api.logger.Error("failed to encode generic error response", zap.Error(err))
	}
}
