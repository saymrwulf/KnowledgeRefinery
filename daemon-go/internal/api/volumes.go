package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

type addVolumeRequest struct {
	Path  string  `json:"path"`
	Label *string `json:"label"`
}

type volumeResponse struct {
	ID         string  `json:"id"`
	Path       string  `json:"path"`
	Label      *string `json:"label"`
	AddedAt    string  `json:"added_at"`
	LastScanAt *string `json:"last_scan_at"`
}

func VolumesRouter(db *storage.Database) chi.Router {
	r := chi.NewRouter()

	r.Post("/add", func(w http.ResponseWriter, r *http.Request) {
		var req addVolumeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		p, _ := filepath.Abs(req.Path)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			http.Error(w, "Not a valid directory: "+req.Path, http.StatusBadRequest)
			return
		}

		idBytes := make([]byte, 8)
		rand.Read(idBytes)
		volID := hex.EncodeToString(idBytes)

		label := req.Label
		if label == nil {
			base := filepath.Base(p)
			label = &base
		}

		vol := storage.NewWatchedVolume(volID, p, label)
		if err := db.AddWatchedVolume(vol); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(volumeResponse{
			ID: vol.ID, Path: vol.Path, Label: vol.Label,
			AddedAt: vol.AddedAt, LastScanAt: vol.LastScanAt,
		})
	})

	r.Get("/list", func(w http.ResponseWriter, r *http.Request) {
		vols, err := db.GetWatchedVolumes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := make([]volumeResponse, len(vols))
		for i, v := range vols {
			resp[i] = volumeResponse{
				ID: v.ID, Path: v.Path, Label: v.Label,
				AddedAt: v.AddedAt, LastScanAt: v.LastScanAt,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	r.Delete("/remove", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		p, _ := filepath.Abs(path)
		if err := db.RemoveWatchedVolume(p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "removed", "path": p})
	})

	return r
}
