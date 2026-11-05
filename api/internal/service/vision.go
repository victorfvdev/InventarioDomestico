package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type VisionService struct {
	repo      *repository.Repository
	apiKey    string
	model     string
	uploadDir string
	client    *http.Client
}

func NewVisionService(repo *repository.Repository, apiKey, model, uploadDir string) *VisionService {
	_ = os.MkdirAll(uploadDir, 0o755)
	return &VisionService{
		repo:      repo,
		apiKey:    strings.TrimSpace(apiKey),
		model:     model,
		uploadDir: uploadDir,
		client:    &http.Client{Timeout: 90 * time.Second},
	}
}

type InterpretResult struct {
	Interpretation domain.AIInterpretation `json:"interpretation"`
	Provider       string                  `json:"provider"`
	Mocked         bool                    `json:"mocked"`
}

func (s *VisionService) InterpretPhoto(ctx context.Context, householdID uuid.UUID, filename string, imageBytes []byte, contentType string) (InterpretResult, error) {
	safeName := fmt.Sprintf("%s_%s", uuid.NewString(), filepath.Base(filename))
	path := filepath.Join(s.uploadDir, safeName)
	if err := os.WriteFile(path, imageBytes, 0o644); err != nil {
		return InterpretResult{}, fmt.Errorf("salvar imagem: %w", err)
	}

	var (
		suggestions []domain.VisionSuggestion
		provider    string
		raw         json.RawMessage
		mocked      bool
		overall     float64
		err         error
	)

	if s.apiKey == "" {
		suggestions, overall = mockSuggestions(filename)
		provider = "local-mock"
		mocked = true
		raw, _ = json.Marshal(map[string]any{
			"note":     "OPENAI_API_KEY ausente — sugestões mock para desenvolvimento",
			"filename": filename,
		})
	} else {
		suggestions, overall, raw, err = s.callOpenAI(ctx, imageBytes, contentType)
		provider = "openai:" + s.model
		if err != nil {
			// Fallback graceful to mock so the flow remains usable
			suggestions, overall = mockSuggestions(filename)
			provider = "local-mock-fallback"
			mocked = true
			raw, _ = json.Marshal(map[string]any{
				"error":    err.Error(),
				"fallback": true,
			})
			err = nil
		}
	}

	interp := domain.AIInterpretation{
		ID:                uuid.New(),
		HouseholdID:       householdID,
		SourceType:        "vision_photo",
		ImageFilename:     safeName,
		Provider:          provider,
		Suggestions:       suggestions,
		Decision:          "pending",
		OverallConfidence: &overall,
	}
	interp, err = s.repo.InsertInterpretation(ctx, interp, raw)
	if err != nil {
		return InterpretResult{}, err
	}
	interp.Suggestions = suggestions

	return InterpretResult{
		Interpretation: interp,
		Provider:       provider,
		Mocked:         mocked,
	}, nil
}

func mockSuggestions(filename string) ([]domain.VisionSuggestion, float64) {
	base := strings.ToLower(filename)
	items := []domain.VisionSuggestion{
		{TempID: uuid.NewString(), Description: "Leite integral 1L", Category: "laticínios", Unit: "un", Quantity: decimal.NewFromInt(2), Confidence: 0.72, Notes: "Embalagens parcialmente ocultas", Included: true},
		{TempID: uuid.NewString(), Description: "Ovos", Category: "proteínas", Unit: "un", Quantity: decimal.NewFromInt(12), Confidence: 0.81, Notes: "Cartela visível", Included: true},
		{TempID: uuid.NewString(), Description: "Arroz branco 5kg", Category: "grãos", Unit: "un", Quantity: decimal.NewFromInt(1), Confidence: 0.64, Notes: "Estimativa — confirme o peso", Included: true},
		{TempID: uuid.NewString(), Description: "Feijão carioca 1kg", Category: "grãos", Unit: "un", Quantity: decimal.NewFromInt(2), Confidence: 0.58, Notes: "Pacotes no fundo da prateleira", Included: true},
		{TempID: uuid.NewString(), Description: "Azeite de oliva", Category: "temperos", Unit: "un", Quantity: decimal.NewFromFloat(0.5), Confidence: 0.49, Notes: "Garrafa parcialmente usada — quantidade estimada", Included: true},
	}
	if strings.Contains(base, "geladeira") || strings.Contains(base, "fridge") {
		items = append(items, domain.VisionSuggestion{
			TempID: uuid.NewString(), Description: "Iogurte natural", Category: "laticínios", Unit: "un",
			Quantity: decimal.NewFromInt(4), Confidence: 0.67, Notes: "Prateleira superior", Included: true,
		})
	}
	return items, 0.66
}

type openAIRequest struct {
	Model    string        `json:"model"`
	Messages []openAIMsg   `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
}

type openAIMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *VisionService) callOpenAI(ctx context.Context, imageBytes []byte, contentType string) ([]domain.VisionSuggestion, float64, json.RawMessage, error) {
	if contentType == "" {
		contentType = "image/jpeg"
	}
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	prompt := `Você analisa fotos de despensa/geladeira para inventário doméstico.
Retorne APENAS JSON válido no formato:
{"overallConfidence":0.0,"items":[{"description":"","category":"","unit":"un|kg|g|L|ml","quantity":0,"confidence":0.0,"notes":""}]}
Regras: estime com cautela; marque confiança baixa quando houver oclusão; não invente marcas sem evidência; quantidade nunca é certeza.`

	body, _ := json.Marshal(openAIRequest{
		Model:     s.model,
		MaxTokens: 1200,
		Messages: []openAIMsg{{
			Role: "user",
			Content: []map[string]any{
				{"type": "text", "text": prompt},
				{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
			},
		}},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	var parsed openAIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, 0, raw, fmt.Errorf("decode openai: %w", err)
	}
	if parsed.Error != nil {
		return nil, 0, raw, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, 0, raw, fmt.Errorf("openai sem choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var payload struct {
		OverallConfidence float64 `json:"overallConfidence"`
		Items             []struct {
			Description string  `json:"description"`
			Category    string  `json:"category"`
			Unit        string  `json:"unit"`
			Quantity    float64 `json:"quantity"`
			Confidence  float64 `json:"confidence"`
			Notes       string  `json:"notes"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, 0, raw, fmt.Errorf("parse vision json: %w", err)
	}

	suggestions := make([]domain.VisionSuggestion, 0, len(payload.Items))
	for _, item := range payload.Items {
		if strings.TrimSpace(item.Description) == "" {
			continue
		}
		unit := item.Unit
		if unit == "" {
			unit = "un"
		}
		cat := item.Category
		if cat == "" {
			cat = "geral"
		}
		suggestions = append(suggestions, domain.VisionSuggestion{
			TempID:      uuid.NewString(),
			Description: item.Description,
			Category:    cat,
			Unit:        unit,
			Quantity:    decimal.NewFromFloat(item.Quantity),
			Confidence:  item.Confidence,
			Notes:       item.Notes,
			Included:    true,
		})
	}
	if len(suggestions) == 0 {
		return nil, 0, raw, fmt.Errorf("nenhum item reconhecido")
	}
	overall := payload.OverallConfidence
	if overall == 0 {
		overall = 0.5
	}
	return suggestions, overall, raw, nil
}
