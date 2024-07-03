-- name: InsertTrip :one
INSERT
INTO trips
    ( "destination", "owner_email", "owner_name", "starts_at", "ends_at") VALUES
    ( $1, $2, $3, $4, $5 )
RETURNING "id";

-- name: GetTrip :one
SELECT
    "id", "destination", "owner_email", "owner_name", "is_confirmed", "starts_at", "ends_at"
FROM trips
WHERE
    id = $1;

-- name: UpdateTrip :exec
UPDATE trips
SET 
    "destination" = $1,
    "ends_at" = $2,
    "starts_at" = $3,
    "is_confirmed" = $4
WHERE
    id = $5;

-- name: GetParticipant :one
SELECT
    "id", "trip_id", "email", "is_confirmed"
FROM participants
WHERE
    id = $1;

-- name: ConfirmParticipant :exec
SELECT
    "id", "trip_id", "email", "is_confirmed"
FROM participants
WHERE
    id = $1;


-- name: GetParticipants :many
SELECT
    "id", "trip_id", "email", "is_confirmed"
FROM participants
WHERE
    trip_id = $1;

-- name: InviteParticipantsToTrip :copyfrom
INSERT INTO participants
    ( "trip_id", "email" ) VALUES
    ( $1, $2 );

-- name: CreateActivity :one
INSERT INTO activities
    ( "trip_id", "title", "occurs_at" ) VALUES
    ( $1, $2, $3 )
RETURNING "id";

-- name: GetTripActivities :many
SELECT
    "id", "trip_id", "title", "occurs_at"
FROM activities
WHERE
    trip_id = $1;

-- name: CreateTripLink :one
INSERT INTO links
    ( "trip_id", "title", "url" ) VALUES
    ( $1, $2, $3 )
RETURNING "id";

-- name: GetTripLinks :many
SELECT
    "id", "trip_id", "title", "url"
FROM links
WHERE
    trip_id = $1;


