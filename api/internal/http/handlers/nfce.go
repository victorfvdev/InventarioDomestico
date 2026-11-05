package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/nfce"
	"github.com/estoque/api/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) NFCeLookup(w http.ResponseWriter, r *http.Request) {
	var in domain.NFCeLookupInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	result, err := a.Purchase.Lookup(r.Context(), a.CurrentSession.Household.ID, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) NFCeFixtureInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"accessKey": nfce.FixtureAccessKey,
		"uf":        "MT",
		"city":      "Cuiabá",
		"issuer":    "ATACADAO S.A.",
		"qrUrl":     "https://www.sefaz.mt.gov.br/nfce/consultanfce?p=" + nfce.FixtureAccessKey + "|2|1|1|A1B2C3D4",
		"howTo":     "POST /api/nfce/lookup com {\"useFixture\":true} ou accessKey da fixture.",
		"sefazLimits": []string{
			"CAPTCHA/sessão bloqueiam automação com frequência",
			"HTML sem schema estável",
			"Fixture offline é o caminho suportado para demos/testes",
		},
	})
}

func (a *API) NFCeManual(w http.ResponseWriter, r *http.Request) {
	var in domain.ManualPurchaseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	doc, err := a.Purchase.ManualDraft(r.Context(), a.CurrentSession.Household.ID, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"document": doc})
}

func (a *API) NFCePhotoStub(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "envie multipart com campo photo (stub local, sem OpenAI)")
		return
	}
	file, header, err := r.FormFile("photo")
	filename := "cupom.jpg"
	if err == nil {
		defer file.Close()
		filename = header.Filename
		_, _ = io.Copy(io.Discard, io.LimitReader(file, 12<<20))
	}
	doc, err := a.Purchase.PhotoStub(r.Context(), a.CurrentSession.Household.ID, filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"document": doc,
		"mocked":   true,
		"provider": "local-photo-stub",
		"message":  "Stub local sem OpenAI — revise os itens antes de confirmar.",
	})
}

func (a *API) GetFiscalDocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	doc, err := a.Purchase.GetDocument(r.Context(), a.CurrentSession.Household.ID, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": doc})
}

func (a *API) ConfirmNFCe(w http.ResponseWriter, r *http.Request) {
	var in domain.ConfirmPurchaseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if in.DocumentID == uuid.Nil {
		if idStr := chi.URLParam(r, "id"); idStr != "" {
			id, err := uuid.Parse(idStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "documentId inválido")
				return
			}
			in.DocumentID = id
		}
	}
	doc, movements, err := a.Purchase.Confirm(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		msg := err.Error()
		if strings.Contains(msg, "já confirmado") || strings.Contains(msg, "já existe entrada") {
			status = http.StatusConflict
		}
		writeError(w, status, msg)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"document":  doc,
		"movements": movements,
		"message":   "Compra confirmada. Movimentações purchase_entry registradas.",
	})
}
