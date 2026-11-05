package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/repository"
	"github.com/estoque/api/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type API struct {
	Repo           *repository.Repository
	Inventory      *service.InventoryService
	Vision         *service.VisionService
	Purchase       *service.PurchaseService
	CurrentSession domain.SessionInfo
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) GetSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.CurrentSession)
}

func (a *API) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := a.Inventory.ListProducts(r.Context(), a.CurrentSession.Household.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": products})
}

func (a *API) ListMovements(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	movements, err := a.Inventory.ListMovements(r.Context(), a.CurrentSession.Household.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"movements": movements})
}

func (a *API) ManualEntry(w http.ResponseWriter, r *http.Request) {
	var in domain.ManualEntryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	product, movement, err := a.Inventory.ManualEntry(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"product": product, "movement": movement})
}

func (a *API) InventoryAdjust(w http.ResponseWriter, r *http.Request) {
	var in domain.InventoryAdjustInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if in.ProductID == uuid.Nil {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "productId inválido")
			return
		}
		in.ProductID = id
	}
	product, movement, err := a.Inventory.InventoryAdjust(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": product, "movement": movement})
}

func (a *API) InterpretPhoto(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "envie um arquivo de imagem (multipart field: photo)")
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "campo photo é obrigatório")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "falha ao ler imagem")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "imagem vazia")
		return
	}

	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(ct, "image/") {
		writeError(w, http.StatusBadRequest, "arquivo precisa ser uma imagem")
		return
	}

	result, err := a.Vision.InterpretPhoto(r.Context(), a.CurrentSession.Household.ID, header.Filename, data, ct)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) GetInterpretation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	interp, err := a.Repo.GetInterpretation(r.Context(), a.CurrentSession.Household.ID, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, interp)
}

func (a *API) ConfirmInitialLoad(w http.ResponseWriter, r *http.Request) {
	var in domain.ConfirmInitialLoadInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	products, movements, err := a.Inventory.ConfirmInitialLoad(r.Context(), a.CurrentSession.Household.ID, a.CurrentSession.User.ID, in)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, repository.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"products":  products,
		"movements": movements,
		"message":   "Inventário inicial confirmado. Quantidades passam a ser confirmadas.",
	})
}
