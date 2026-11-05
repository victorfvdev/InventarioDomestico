package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
)

func (a *API) InterpretConsumption(w http.ResponseWriter, r *http.Request) {
	var in domain.InterpretConsumptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	result, err := a.Consumption.Interpret(r.Context(), a.CurrentSession.Household.ID, in.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ConfirmConsumption(w http.ResponseWriter, r *http.Request) {
	var in domain.ConfirmConsumptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	movements, err := a.Consumption.Confirm(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"movements": movements,
		"message":   "Consumo registrado. Ajuste de inventário continua separado de consumo.",
	})
}

func (a *API) ListConsumptionShortcuts(w http.ResponseWriter, r *http.Request) {
	shortcuts, err := a.Consumption.Shortcuts(r.Context(), a.CurrentSession.Household.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shortcuts": shortcuts})
}

func (a *API) QuickConsume(w http.ResponseWriter, r *http.Request) {
	var in domain.QuickConsumeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	movement, err := a.Consumption.Quick(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"movement": movement,
		"message":  "Consumo rápido confirmado",
	})
}

func (a *API) UndoConsumption(w http.ResponseWriter, r *http.Request) {
	var in domain.UndoConsumptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if in.MovementID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "movementId é obrigatório")
		return
	}
	movement, err := a.Consumption.Undo(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in.MovementID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"movement": movement,
		"message":  "Consumo desfeito via ajuste (não conta como consumo)",
	})
}
