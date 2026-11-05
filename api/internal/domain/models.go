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

type FiscalDocStatus string

const (
	FiscalDraft     FiscalDocStatus = "draft"
	FiscalFetched   FiscalDocStatus = "fetched"
	FiscalParsed    FiscalDocStatus = "parsed"
	FiscalConfirmed FiscalDocStatus = "confirmed"
	FiscalRejected  FiscalDocStatus = "rejected"
)

type MatchStatus string

const (
	MatchExact      MatchStatus = "exact"
	MatchSuggested  MatchStatus = "suggested"
	MatchAmbiguous  MatchStatus = "ambiguous"
	MatchNew        MatchStatus = "new"
	MatchManual     MatchStatus = "manual"
)

type FiscalDocument struct {
	ID          uuid.UUID       `json:"id"`
	HouseholdID uuid.UUID       `json:"householdId"`
	AccessKey   string          `json:"accessKey"`
	UF          string          `json:"uf"`
	DocumentType string         `json:"documentType"`
	IssuerName  string          `json:"issuerName"`
	IssuerCNPJ  string          `json:"issuerCnpj"`
	IssuedAt    *time.Time      `json:"issuedAt,omitempty"`
	TotalAmount decimal.Decimal `json:"totalAmount"`
	Status      FiscalDocStatus `json:"status"`
	Source      string          `json:"source"`
	FetchNotes  string          `json:"fetchNotes,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	ConfirmedAt *time.Time      `json:"confirmedAt,omitempty"`
	Items       []FiscalItem    `json:"items,omitempty"`
	AlreadyPosted bool          `json:"alreadyPosted,omitempty"`
}

type FiscalItem struct {
	ID                    uuid.UUID        `json:"id"`
	FiscalDocumentID      uuid.UUID        `json:"fiscalDocumentId"`
	LineNumber            int              `json:"lineNumber"`
	Description           string           `json:"description"`
	DescriptionNormalized string           `json:"descriptionNormalized"`
	Quantity              decimal.Decimal  `json:"quantity"`
	Unit                  string           `json:"unit"`
	UnitPrice             decimal.Decimal  `json:"unitPrice"`
	TotalPrice            decimal.Decimal  `json:"totalPrice"`
	EAN                   string           `json:"ean,omitempty"`
	Category              string           `json:"category"`
	MatchStatus           MatchStatus      `json:"matchStatus"`
	MatchConfidence       *float64         `json:"matchConfidence,omitempty"`
	MatchedProductID      *uuid.UUID       `json:"matchedProductId,omitempty"`
	MatchedProductName    string           `json:"matchedProductName,omitempty"`
	CreateNew             bool             `json:"createNew"`
	Included              bool             `json:"included"`
	AmbiguousCandidates   []ProductMatch   `json:"ambiguousCandidates,omitempty"`
}

type ProductMatch struct {
	ProductID   uuid.UUID `json:"productId"`
	Description string    `json:"description"`
	Score       float64   `json:"score"`
}

type NFCeLookupInput struct {
	AccessKey     string `json:"accessKey"`
	QRUrl         string `json:"qrUrl"`
	UseFixture    bool   `json:"useFixture"`
}

type ManualPurchaseItem struct {
	Description string          `json:"description"`
	Quantity    decimal.Decimal `json:"quantity"`
	Unit        string          `json:"unit"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
	Category    string          `json:"category"`
	EAN         string          `json:"ean"`
}

type ManualPurchaseInput struct {
	AccessKey  string               `json:"accessKey"`
	IssuerName string               `json:"issuerName"`
	Items      []ManualPurchaseItem `json:"items"`
	Notes      string               `json:"notes"`
	Source     string               `json:"source,omitempty"` // manual | photo_stub
}

type ConfirmPurchaseItem struct {
	ItemID      uuid.UUID       `json:"itemId"`
	Included    bool            `json:"included"`
	Description string          `json:"description"`
	Quantity    decimal.Decimal `json:"quantity"`
	Unit        string          `json:"unit"`
	Category    string          `json:"category"`
	ProductID   *uuid.UUID      `json:"productId"`
	CreateNew   bool            `json:"createNew"`
}

type ConfirmPurchaseInput struct {
	DocumentID uuid.UUID             `json:"documentId"`
	Items      []ConfirmPurchaseItem `json:"items"`
}

// --- Consumption / natural language (RN04, RN06) ---

type ConsumptionLineSuggestion struct {
	TempID               string          `json:"tempId"`
	MentionedText        string          `json:"mentionedText"`
	ProductHint          string          `json:"productHint"`
	Quantity             decimal.Decimal `json:"quantity"`
	Unit                 string          `json:"unit"`
	QuantityInferred     bool            `json:"quantityInferred"`
	Approx               bool            `json:"approx"`
	MatchStatus          MatchStatus     `json:"matchStatus"`
	MatchedProductID     *uuid.UUID      `json:"matchedProductId,omitempty"`
	MatchedProductName   string          `json:"matchedProductName,omitempty"`
	MatchConfidence      *float64        `json:"matchConfidence,omitempty"`
	AvailableQuantity    decimal.Decimal `json:"availableQuantity"`
	InsufficientStock    bool            `json:"insufficientStock"`
	Candidates           []ProductMatch  `json:"candidates,omitempty"`
	Included             bool            `json:"included"`
	Notes                string          `json:"notes,omitempty"`
	Unmatched            bool            `json:"unmatched"`
}

type ConsumptionInterpretation struct {
	ID              uuid.UUID                   `json:"id"`
	HouseholdID     uuid.UUID                   `json:"householdId"`
	SourceType      string                      `json:"sourceType"`
	Provider        string                      `json:"provider"`
	OriginalText    string                      `json:"originalText"`
	Lines           []ConsumptionLineSuggestion `json:"lines"`
	Decision        string                      `json:"decision"`
	CreatedAt       time.Time                   `json:"createdAt"`
	ConfirmedAt     *time.Time                  `json:"confirmedAt,omitempty"`
}

type InterpretConsumptionInput struct {
	Text string `json:"text"`
}

type InterpretConsumptionResult struct {
	Interpretation ConsumptionInterpretation `json:"interpretation"`
	Provider       string                    `json:"provider"`
	Mocked         bool                      `json:"mocked"`
}

type ConfirmConsumptionItem struct {
	TempID    string          `json:"tempId"`
	Included  bool            `json:"included"`
	ProductID uuid.UUID       `json:"productId"`
	Quantity  decimal.Decimal `json:"quantity"`
	Unit      string          `json:"unit"`
	Notes     string          `json:"notes"`
}

type ConfirmConsumptionInput struct {
	InterpretationID uuid.UUID                `json:"interpretationId"`
	Items            []ConfirmConsumptionItem `json:"items"`
}

type QuickConsumeInput struct {
	ProductID uuid.UUID       `json:"productId"`
	Quantity  decimal.Decimal `json:"quantity"`
	Unit      string          `json:"unit"`
	Notes     string          `json:"notes"`
	Confirm   bool            `json:"confirm"`
}

type UndoConsumptionInput struct {
	MovementID uuid.UUID `json:"movementId"`
}

type ConsumptionShortcut struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	ProductID   uuid.UUID       `json:"productId"`
	ProductName string          `json:"productName"`
	Quantity    decimal.Decimal `json:"quantity"`
	Unit        string          `json:"unit"`
	Source      string          `json:"source"` // history | seed
	Available   decimal.Decimal `json:"available"`
}
