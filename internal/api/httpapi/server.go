package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/app"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/media"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/tracking"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/infrastructure/middleware"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/types"
)

type Server struct {
	media     *app.MediaService
	providers *app.ProviderService
	logger    *slog.Logger
}

func NewRouter(mediaSvc *app.MediaService, providerSvc *app.ProviderService, logger *slog.Logger) http.Handler {
	s := &Server{media: mediaSvc, providers: providerSvc, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/videos", s.listVideos)
	mux.HandleFunc("GET /api/providers", s.listProviders)
	mux.HandleFunc("GET /api/secrets", s.listSecrets)
	mux.HandleFunc("GET /api/video/", s.video)
	return middleware.Chain(
		mux,
		middleware.Recover(logger),
		middleware.RequestID(),
		middleware.CORS(),
		middleware.Logger(logger),
	)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := s.media.ListVideos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, types.ListVideosResponse{Videos: videos})
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.providers.ListProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	secrets, err := s.providers.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": secrets})
}

func (s *Server) video(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/video/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		writeErrorText(w, http.StatusNotFound, "not found")
		return
	}
	scene := parts[0]
	action := parts[1]
	switch action {
	case "meta":
		s.videoMeta(w, r, scene)
	case "boxes":
		s.boxes(w, r, scene)
	case "frame":
		if len(parts) < 3 {
			writeErrorText(w, http.StatusNotFound, "frame missing")
			return
		}
		frameText := strings.TrimSuffix(parts[2], ".jpg")
		frame, _ := strconv.Atoi(frameText)
		s.frame(w, r, scene, frame)
	case "preview":
		s.preview(w, r, scene)
	default:
		writeErrorText(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) videoMeta(w http.ResponseWriter, r *http.Request, scene string) {
	v, err := s.media.GetVideo(r.Context(), scene)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, types.VideoMetaResponse{
		Scene:             v.Scene,
		FrameCount:        v.FrameCount,
		Rows:              v.Rows,
		Tracks:            v.Tracks,
		Classes:           classCounts(v),
		AnomalyFrameCount: v.AnomalyFrameCount,
	})
}

func (s *Server) boxes(w http.ResponseWriter, r *http.Request, scene string) {
	frame, _ := strconv.Atoi(r.URL.Query().Get("frame"))
	boxes, err := s.media.GetBoxes(r.Context(), scene, frame)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, types.BoxesResponse{Scene: scene, Frame: frame, Boxes: boxes})
}

func (s *Server) frame(w http.ResponseWriter, r *http.Request, scene string, frame int) {
	f, contentType, err := s.media.OpenFrame(r.Context(), scene, frame)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, scene+".jpg", time.Time{}, f)
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request, scene string) {
	path, err := s.media.PreviewPath(r.Context(), scene)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

func classCounts(v *media.Video) []media.ClassCount {
	out := make([]media.ClassCount, 0, len(v.ClassCounts))
	for id, count := range v.ClassCounts {
		out = append(out, media.ClassCount{
			ClassID:   id,
			ClassName: tracking.ClassName(id),
			Color:     tracking.ClassColor(id),
			Count:     count,
		})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeErrorText(w, status, err.Error())
}

func writeErrorText(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, types.ErrorResponse{Error: msg})
}
