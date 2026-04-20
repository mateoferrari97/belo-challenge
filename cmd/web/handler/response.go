package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mateoferrari97/belo-challenge/internal/transaction"
)

type createResponseBody struct {
	ID            uuid.UUID          `json:"id"             swaggertype:"string" format:"uuid"     example:"550e8400-e29b-41d4-a716-446655440002"`
	SourceID      uuid.UUID          `json:"source_id"      swaggertype:"string" format:"uuid"     example:"550e8400-e29b-41d4-a716-446655440000"`
	DestinationID uuid.UUID          `json:"destination_id" swaggertype:"string" format:"uuid"     example:"550e8400-e29b-41d4-a716-446655440001"`
	Amount        decimal.Decimal    `json:"amount"         swaggertype:"string" format:"decimal"  example:"100.50"`
	Status        transaction.Status `json:"status"         enums:"approved,pending,rejected"          example:"approved"`
	CreatedAt     time.Time          `json:"created_at"                                             example:"2026-04-19T12:00:00Z"`
	UpdatedAt     time.Time          `json:"updated_at"                                             example:"2026-04-19T12:00:00Z"`
}

type listResponseBody struct {
	Data       []createResponseBody `json:"data"`
	NextCursor *string              `json:"next_cursor" example:"eyJjcmVhdGVkX2F0IjoiMjAyNi0wNC0xOVQxMjowMDowMFoiLCJpZCI6IjU1MGU4NDAwLWUyOWItNDFkNC1hNzE2LTQ0NjY1NTQ0MDAwMCJ9"`
}

func toCreateResponseBody(t transaction.Transaction) createResponseBody {
	return createResponseBody{
		ID:            t.ID,
		SourceID:      t.SourceID,
		DestinationID: t.DestinationID,
		Amount:        t.Amount,
		Status:        t.Status,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

func toListResponseBody(out transaction.ListOutput) (listResponseBody, error) {
	data := make([]createResponseBody, 0, len(out.Transactions))
	for _, t := range out.Transactions {
		data = append(data, toCreateResponseBody(t))
	}

	var next *string
	if out.NextCursor != nil {
		encoded, err := encodeCursor(*out.NextCursor)
		if err != nil {
			return listResponseBody{}, err
		}
		next = &encoded
	}

	return listResponseBody{Data: data, NextCursor: next}, nil
}

func encodeCursor(cursor transaction.Cursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
}
