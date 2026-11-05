package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ConfirmationStatus string

const (
	StatusEstimated ConfirmationStatus = "estimated"
	StatusConfirmed ConfirmationStatus = "confirmed"
)

type MovementType string

const (
	MovementManualEntry      MovementType = "manual_entry"
	MovementInventoryAdjust  MovementType = "inventory_adjust"
	MovementInitialLoad      MovementType = "initial_load"
	MovementPurchaseEntry    MovementType = "purchase_entry"
	MovementConsumption      MovementType = "consumption"
	MovementLoss             MovementType = "loss"
)

type Household struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	HouseholdID  uuid.UUID `json:"householdId"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Product struct {
	ID                   uuid.UUID          `json:"id"`
	HouseholdID          uuid.UUID          `json:"householdId"`
	Description          string             `json:"description"`
	DescriptionNormalized string            `json:"descriptionNormalized"`
	Category             string             `json:"category"`
	Unit                 string             `json:"unit"`
	Quantity             decimal.Decimal    `json:"quantity"`
	MinQuantity          *decimal.Decimal   `json:"minQuantity,omitempty"`
	ConfirmationStatus   ConfirmationStatus `json:"confirmationStatus"`
	CreatedAt            time.Time          `json:"createdAt"`
	UpdatedAt            time.Time          `json:"updatedAt"`
}

type Movement struct {
	ID             uuid.UUID       `json:"id"`
	HouseholdID    uuid.UUID       `json:"householdId"`
	ProductID      uuid.UUID       `json:"productId"`
	MovementType   MovementType    `json:"movementType"`
	QuantityDelta  decimal.Decimal `json:"quantityDelta"`
	BalanceAfter   decimal.Decimal `json:"balanceAfter"`
	Origin         string          `json:"origin"`
	Notes          string          `json:"notes,omitempty"`
	IsConsumption  bool            `json:"isConsumption"`
	CreatedBy      *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	ProductName    string          `json:"productName,omitempty"`
}

type VisionSuggestion struct {
	TempID      string          `json:"tempId"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Unit        string          `json:"unit"`
	Quantity    decimal.Decimal `json:"quantity"`
	Confidence  float64         `json:"confidence"`
	Notes       string          `json:"notes,omitempty"`
	Included    bool            `json:"included"`
}

type AIInterpretation struct {
	ID                uuid.UUID          `json:"id"`
	HouseholdID       uuid.UUID          `json:"householdId"`
	SourceType        string             `json:"sourceType"`
	ImageFilename     string             `json:"imageFilename,omitempty"`
	Provider          string             `json:"provider"`
	Suggestions       []VisionSuggestion `json:"suggestions"`
	Decision          string             `json:"decision"`
	OverallConfidence *float64           `json:"overallConfidence,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
	ConfirmedAt       *time.Time         `json:"confirmedAt,omitempty"`
}

type ManualEntryInput struct {
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Unit        string           `json:"unit"`
	Quantity    decimal.Decimal  `json:"quantity"`
	MinQuantity *decimal.Decimal `json:"minQuantity,omitempty"`
	Notes       string           `json:"notes"`
}

type InventoryAdjustInput struct {
	ProductID   uuid.UUID       `json:"productId"`
	NewQuantity decimal.Decimal `json:"newQuantity"`
	Notes       string          `json:"notes"`
}

type ConfirmInitialLoadItem struct {
	TempID      string           `json:"tempId"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Unit        string           `json:"unit"`
	Quantity    decimal.Decimal  `json:"quantity"`
	MinQuantity *decimal.Decimal `json:"minQuantity,omitempty"`
	Included    bool             `json:"included"`
	Notes       string           `json:"notes"`
}

type ConfirmInitialLoadInput struct {
	InterpretationID uuid.UUID                 `json:"interpretationId"`
	Items            []ConfirmInitialLoadItem  `json:"items"`
}

type SessionInfo struct {
	User      User      `json:"user"`
	Household Household `json:"household"`
}
