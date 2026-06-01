package httpapi

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentapp"
	"github.com/qsyy0921/automated_training_model/internal/app/annotationapp"
	"github.com/qsyy0921/automated_training_model/internal/app/datasetapp"
	"github.com/qsyy0921/automated_training_model/internal/app/lifecycleapp"
	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/app/providerapp"
	"github.com/qsyy0921/automated_training_model/internal/app/workspaceapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/annotation"
	"github.com/qsyy0921/automated_training_model/internal/domain/dataset"
	"github.com/qsyy0921/automated_training_model/internal/domain/media"
	"github.com/qsyy0921/automated_training_model/internal/domain/taxonomy"
	"github.com/qsyy0921/automated_training_model/internal/domain/tracking"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/middleware"
	"github.com/qsyy0921/automated_training_model/internal/types"
)

type Server struct {
	media       *mediaapp.MediaService
	annotations *annotationapp.AnnotationService
	datasets    *datasetapp.DatasetService
	workspace   *workspaceapp.RuntimeService
	lifecycle   *lifecycleapp.Service
	agents      *agentapp.Service
	providers   *providerapp.ProviderService
	taxonomy    taxonomy.Taxonomy
	webRoot     string
	dataRoot    string
	logger      *slog.Logger
}

func NewRouter(mediaSvc *mediaapp.MediaService, annotationSvc *annotationapp.AnnotationService, datasetSvc *datasetapp.DatasetService, workspaceSvc *workspaceapp.RuntimeService, lifecycleSvc *lifecycleapp.Service, agentSvc *agentapp.Service, providerSvc *providerapp.ProviderService, taxonomyCfg taxonomy.Taxonomy, webRoot string, dataRoot string, logger *slog.Logger) http.Handler {
	s := &Server{media: mediaSvc, annotations: annotationSvc, datasets: datasetSvc, workspace: workspaceSvc, lifecycle: lifecycleSvc, agents: agentSvc, providers: providerSvc, taxonomy: taxonomyCfg.FillDefaults(), webRoot: webRoot, dataRoot: dataRoot, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /", s.index)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(s.staticWebRoot(), "assets")))))
	mux.HandleFunc("GET /api/videos", s.listVideos)
	mux.HandleFunc("GET /api/taxonomy", s.taxonomyHandler)
	mux.HandleFunc("GET /api/datasets", s.listDatasets)
	mux.HandleFunc("POST /api/datasets/register-folder", s.registerFolderDataset)
	mux.HandleFunc("POST /api/datasets/register-manifest", s.registerManifestDataset)
	mux.HandleFunc("POST /api/datasets/upload-archive", s.uploadArchiveDataset)
	mux.HandleFunc("POST /api/datasets/", s.datasetAction)
	mux.HandleFunc("GET /api/providers", s.listProviders)
	mux.HandleFunc("GET /api/secrets", s.listSecrets)
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.saveAgent)
	mux.HandleFunc("GET /api/agents/", s.agentDetail)
	mux.HandleFunc("GET /api/tools", s.listAgentTools)
	mux.HandleFunc("POST /api/tools", s.saveAgentTool)
	mux.HandleFunc("GET /api/workflows", s.listAgentWorkflows)
	mux.HandleFunc("POST /api/workflows", s.saveAgentWorkflow)
	mux.HandleFunc("GET /api/workflows/", s.agentWorkflowDetail)
	mux.HandleFunc("GET /api/agent-runs", s.listAgentRuns)
	mux.HandleFunc("POST /api/agent-runs", s.submitAgentRun)
	mux.HandleFunc("GET /api/audit-events", s.listAuditEvents)
	mux.HandleFunc("GET /api/governance/enforcement-points", s.listEnforcementPoints)
	mux.HandleFunc("GET /api/governance/data-policies", s.listDataGovernancePolicies)
	mux.HandleFunc("GET /api/governance/release-policies", s.listReleasePolicies)
	mux.HandleFunc("GET /api/governance/runtime-policies", s.listRuntimePolicies)
	mux.HandleFunc("GET /api/governance/control-surface", s.getControlSurface)
	mux.HandleFunc("POST /api/autolabel/jobs", s.submitAutoLabel)
	mux.HandleFunc("POST /api/training/runs", s.submitTraining)
	mux.HandleFunc("POST /api/evaluation/runs", s.submitEvaluation)
	mux.HandleFunc("GET /api/models", s.listModels)
	mux.HandleFunc("GET /api/models/", s.modelDetail)
	mux.HandleFunc("POST /api/models/register", s.registerModel)
	mux.HandleFunc("POST /api/deployments", s.submitDeployment)
	mux.HandleFunc("GET /api/tasks/", s.taskStatus)
	mux.HandleFunc("DELETE /api/tasks/", s.cancelTask)
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

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeErrorText(w, http.StatusNotFound, "not found")
		return
	}
	path := filepath.Join(s.staticWebRoot(), "index.html")
	if _, err := os.Stat(path); err == nil {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, path)
		return
	}
	writeErrorText(w, http.StatusNotFound, "web/index.html not found")
}

func (s *Server) staticWebRoot() string {
	distRoot := filepath.Join(s.webRoot, "dist")
	if _, err := os.Stat(filepath.Join(distRoot, "index.html")); err == nil {
		return distRoot
	}
	return s.webRoot
}

func (s *Server) listVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := s.media.ListVideos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for i := range videos {
		anns, err := s.annotations.List(r.Context(), videos[i].Scene)
		if err == nil {
			videos[i].AnnotationCount = len(anns)
		}
		v, err := s.media.GetVideo(r.Context(), videos[i].Scene)
		if err != nil {
			continue
		}
		rejected, _ := s.annotations.RejectedTrackKeys(r.Context(), videos[i].Scene)
		videos[i].TrackCount = len(filterTracks(v.Tracks, rejected))
		videos[i].Classes, videos[i].Rows = classCountsFromBoxes(v, rejected)
	}
	writeJSON(w, http.StatusOK, types.ListVideosResponse{Videos: videos})
}

func (s *Server) taxonomyHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.taxonomy)
}

func (s *Server) listDatasets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.datasets.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"datasets": rows})
}

func (s *Server) registerFolderDataset(w http.ResponseWriter, r *http.Request) {
	var req dataset.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.SourceType = dataset.SourceLocalFolder
	ds, err := s.datasets.Register(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"dataset": ds})
}

func (s *Server) registerManifestDataset(w http.ResponseWriter, r *http.Request) {
	var req dataset.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.SourceType = dataset.SourceManifest
	ds, err := s.datasets.Register(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"dataset": ds})
}

func (s *Server) uploadArchiveDataset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	uploadDir := filepath.Join(s.dataRoot, "uploads", time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	target := filepath.Join(uploadDir, filepath.Base(header.Filename))
	out, err := os.Create(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := out.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	extractRoot := ""
	mergeRoot, frameRoot, maskRoot := "", "", ""
	if strings.EqualFold(filepath.Ext(target), ".zip") {
		extractRoot = filepath.Join(uploadDir, "extracted")
		if err := extractZip(target, extractRoot); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		mergeRoot, frameRoot, maskRoot = inferDatasetRoots(extractRoot)
	}
	ds, err := s.datasets.Register(r.Context(), dataset.RegisterRequest{
		Name:       name,
		SourceType: dataset.SourceUpload,
		UploadPath: target,
		MergeRoot:  mergeRoot,
		FrameRoot:  frameRoot,
		MaskRoot:   maskRoot,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"dataset": ds, "extract_root": extractRoot})
}

func (s *Server) datasetAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/datasets/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[1] != "activate" {
		writeErrorText(w, http.StatusNotFound, "not found")
		return
	}
	ds, err := s.workspace.Activate(r.Context(), parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dataset": ds, "active": true})
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
	case "annotations":
		s.annotationsHandler(w, r, scene)
	case "annotation":
		if len(parts) < 3 {
			writeErrorText(w, http.StatusNotFound, "annotation id missing")
			return
		}
		s.annotationHandler(w, r, scene, parts[2])
	case "purge-tracks":
		s.purgeTracks(w, r, scene)
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
	rejected, _ := s.annotations.RejectedTrackKeys(r.Context(), scene)
	classes, rows := classCountsFromBoxes(v, rejected)
	writeJSON(w, http.StatusOK, types.VideoMetaResponse{
		Scene:             v.Scene,
		FrameCount:        v.FrameCount,
		Rows:              rows,
		Tracks:            filterTracks(v.Tracks, rejected),
		Classes:           classes,
		AnomalyFrameCount: v.AnomalyFrameCount,
		AnomalySegments:   v.AnomalySegments,
		Annotations:       s.mustAnnotations(r, scene),
	})
}

func (s *Server) boxes(w http.ResponseWriter, r *http.Request, scene string) {
	frame, _ := strconv.Atoi(r.URL.Query().Get("frame"))
	boxes, err := s.media.GetBoxes(r.Context(), scene, frame)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	rejected, _ := s.annotations.RejectedTrackKeys(r.Context(), scene)
	boxes = filterBoxes(boxes, rejected)
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
	w.Header().Set("Cache-Control", "public, max-age=3600")
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

func (s *Server) annotationsHandler(w http.ResponseWriter, r *http.Request, scene string) {
	switch r.Method {
	case http.MethodGet:
		anns, err := s.annotations.List(r.Context(), scene)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, anns)
	case http.MethodPost:
		var ann annotation.Annotation
		if err := json.NewDecoder(r.Body).Decode(&ann); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		saved, err := s.annotations.Save(r.Context(), scene, ann)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"annotation": saved})
	default:
		writeErrorText(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) annotationHandler(w http.ResponseWriter, r *http.Request, scene string, id string) {
	if r.Method != http.MethodDelete {
		writeErrorText(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.annotations.Delete(r.Context(), scene, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) purgeTracks(w http.ResponseWriter, r *http.Request, scene string) {
	if r.Method != http.MethodPost {
		writeErrorText(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		TrackKeys []string `json:"track_keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	removed, err := s.media.PurgeTracks(r.Context(), scene, req.TrackKeys)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scene":        scene,
		"track_keys":   req.TrackKeys,
		"removed_rows": removed,
	})
}

func classCountsFromBoxes(v *media.Video, rejected map[string]bool) ([]media.ClassCount, int) {
	counts := map[int]int{}
	rows := 0
	for _, boxes := range v.Boxes {
		for _, box := range boxes {
			if rejected[box.TrackKey] {
				continue
			}
			counts[box.ClassID]++
			rows++
		}
	}
	return classCountsFromMap(counts), rows
}

func classCountsFromMap(counts map[int]int) []media.ClassCount {
	out := make([]media.ClassCount, 0, len(counts))
	for id, count := range counts {
		out = append(out, media.ClassCount{
			ClassID:   id,
			ClassName: tracking.ClassName(id),
			Color:     tracking.ClassColor(id),
			Count:     count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ClassID < out[j].ClassID
	})
	return out
}

func (s *Server) mustAnnotations(r *http.Request, scene string) []annotation.Annotation {
	anns, err := s.annotations.List(r.Context(), scene)
	if err != nil {
		return []annotation.Annotation{}
	}
	return anns
}

func (s *Server) filteredTracks(r *http.Request, scene string, v *media.Video) []tracking.Track {
	rejected, _ := s.annotations.RejectedTrackKeys(r.Context(), scene)
	return filterTracks(v.Tracks, rejected)
}

func filterTracks(tracks []tracking.Track, rejected map[string]bool) []tracking.Track {
	if len(rejected) == 0 {
		return tracks
	}
	out := make([]tracking.Track, 0, len(tracks))
	for _, track := range tracks {
		if rejected[track.TrackKey] {
			continue
		}
		out = append(out, track)
	}
	return out
}

func filterBoxes(boxes []tracking.Box, rejected map[string]bool) []tracking.Box {
	if len(rejected) == 0 {
		return boxes
	}
	out := make([]tracking.Box, 0, len(boxes))
	for _, box := range boxes {
		if rejected[box.TrackKey] {
			continue
		}
		out = append(out, box)
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

func extractZip(src string, dst string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	dstClean, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target := filepath.Join(dstClean, file.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(targetAbs, dstClean+string(os.PathSeparator)) && targetAbs != dstClean {
			return fmt.Errorf("unsafe zip path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(targetAbs)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func inferDatasetRoots(root string) (mergeRoot string, frameRoot string, maskRoot string) {
	candidates := []string{root}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if entry.IsDir() {
			candidates = append(candidates, filepath.Join(root, entry.Name()))
		}
	}
	for _, base := range candidates {
		if mergeRoot == "" && fileExists(filepath.Join(base, "csv")) {
			mergeRoot = base
		}
		for _, rel := range []string{"frames", filepath.Join("testing", "frames"), filepath.Join("data", "testing", "frames")} {
			p := filepath.Join(base, rel)
			if frameRoot == "" && fileExists(p) {
				frameRoot = p
			}
		}
		for _, rel := range []string{"testframemask", filepath.Join("data", "testframemask")} {
			p := filepath.Join(base, rel)
			if maskRoot == "" && fileExists(p) {
				maskRoot = p
			}
		}
	}
	return mergeRoot, frameRoot, maskRoot
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
