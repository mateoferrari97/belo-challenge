package handler

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/mateoferrari97/belo-challenge/internal/transaction"
)

type createRequestBody struct {
	SourceID      uuid.UUID       `json:"source_id"      swaggertype:"string" format:"uuid"     example:"550e8400-e29b-41d4-a716-446655440000"`
	DestinationID uuid.UUID       `json:"destination_id" swaggertype:"string" format:"uuid"     example:"550e8400-e29b-41d4-a716-446655440001"`
	Amount        decimal.Decimal `json:"amount"         swaggertype:"string" format:"decimal"  example:"100.50"`
}

func decodeCursor(encoded string) (transaction.Cursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return transaction.Cursor{}, err
	}

	var cursor transaction.Cursor
	if err = json.Unmarshal(payload, &cursor); err != nil {
		return transaction.Cursor{}, err
	}

	return cursor, nil
}
