package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const supportPayloadLimit = 32 << 10

type browserDiagnostics struct {
	UserAgent           string  `json:"user_agent,omitempty"`
	Platform            string  `json:"platform,omitempty"`
	Language            string  `json:"language,omitempty"`
	HardwareConcurrency int     `json:"hardware_concurrency,omitempty"`
	DeviceMemoryGB      float64 `json:"device_memory_gb,omitempty"`
	ScreenWidth         int     `json:"screen_width,omitempty"`
	ScreenHeight        int     `json:"screen_height,omitempty"`
	PixelRatio          float64 `json:"pixel_ratio,omitempty"`
	Online              *bool   `json:"online,omitempty"`
	ConnectionType      string  `json:"connection_type,omitempty"`
	DownlinkMbps        float64 `json:"downlink_mbps,omitempty"`
	SaveData            *bool   `json:"save_data,omitempty"`
}

type playbackDiagnostic struct {
	SessionID                   string  `json:"session_id,omitempty"`
	ItemKind                    string  `json:"item_kind,omitempty"`
	ItemID                      int64   `json:"item_id,omitempty"`
	FileID                      int64   `json:"file_id,omitempty"`
	Mode                        string  `json:"mode,omitempty"`
	ReasonCode                  string  `json:"reason_code,omitempty"`
	VideoCodec                  string  `json:"video_codec,omitempty"`
	AudioCodec                  string  `json:"audio_codec,omitempty"`
	Encoder                     string  `json:"encoder,omitempty"`
	EncoderKind                 string  `json:"encoder_kind,omitempty"`
	HardwareDecoder             string  `json:"hardware_decoder,omitempty"`
	SourceHeight                int     `json:"source_height,omitempty"`
	OutputHeight                int     `json:"output_height,omitempty"`
	FrameRate                   float64 `json:"frame_rate,omitempty"`
	PositionSeconds             float64 `json:"position_seconds,omitempty"`
	BufferedSeconds             float64 `json:"buffered_seconds,omitempty"`
	ReadyState                  int     `json:"ready_state,omitempty"`
	TotalFrames                 uint64  `json:"total_frames,omitempty"`
	DroppedFrames               uint64  `json:"dropped_frames,omitempty"`
	BufferTargetSeconds         float64 `json:"buffer_target_seconds,omitempty"`
	PreviousBufferTargetSeconds float64 `json:"previous_buffer_target_seconds,omitempty"`
	Automatic                   bool    `json:"automatic,omitempty"`
}

type clientDiagnosticRequest struct {
	Event        string             `json:"event"`
	ClientTime   string             `json:"client_time,omitempty"`
	Client       browserDiagnostics `json:"client,omitempty"`
	Playback     playbackDiagnostic `json:"playback,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

type supportExportRequest struct {
	Client browserDiagnostics `json:"client,omitempty"`
}

var clientDiagnosticEvents = map[string]bool{
	"browser_error":            true,
	"playback_plan":            true,
	"playback_ready":           true,
	"playback_waiting":         true,
	"playback_error":           true,
	"playback_buffer_pressure": true,
	"playback_paused_release":  true,
	"quality_adapted":          true,
}

func (s *Server) handleClientDiagnostic(w http.ResponseWriter, r *http.Request) {
	var body clientDiagnosticRequest
	if err := decodeSupportJSON(w, r, &body, false); err != nil || !clientDiagnosticEvents[body.Event] {
		writeJSONError(w, http.StatusBadRequest, "invalid_diagnostic_event")
		return
	}
	normalizeBrowserDiagnostics(&body.Client)
	normalizePlaybackDiagnostic(&body.Playback)
	body.ClientTime = boundedText(body.ClientTime, 64)
	body.ErrorMessage = boundedText(s.sanitizeSupportText(body.ErrorMessage), 500)

	s.log.Info("client diagnostic",
		"event", body.Event,
		"client_time", body.ClientTime,
		browserLogGroup(body.Client),
		playbackLogGroup(body.Playback),
		"error_message", body.ErrorMessage,
	)
	w.WriteHeader(http.StatusNoContent)
}

func browserLogGroup(client browserDiagnostics) slog.Attr {
	return slog.Group("browser",
		"user_agent", client.UserAgent,
		"platform", client.Platform,
		"language", client.Language,
		"hardware_concurrency", client.HardwareConcurrency,
		"device_memory_gb", client.DeviceMemoryGB,
		"screen_width", client.ScreenWidth,
		"screen_height", client.ScreenHeight,
		"pixel_ratio", client.PixelRatio,
		"online", client.Online,
		"connection_type", client.ConnectionType,
		"downlink_mbps", client.DownlinkMbps,
		"save_data", client.SaveData,
	)
}

func playbackLogGroup(playback playbackDiagnostic) slog.Attr {
	return slog.Group("playback",
		"session_id", playback.SessionID,
		"item_kind", playback.ItemKind,
		"item_id", playback.ItemID,
		"file_id", playback.FileID,
		"mode", playback.Mode,
		"reason_code", playback.ReasonCode,
		"video_codec", playback.VideoCodec,
		"audio_codec", playback.AudioCodec,
		"encoder", playback.Encoder,
		"encoder_kind", playback.EncoderKind,
		"hardware_decoder", playback.HardwareDecoder,
		"source_height", playback.SourceHeight,
		"output_height", playback.OutputHeight,
		"frame_rate", playback.FrameRate,
		"position_seconds", playback.PositionSeconds,
		"buffered_seconds", playback.BufferedSeconds,
		"ready_state", playback.ReadyState,
		"total_frames", playback.TotalFrames,
		"dropped_frames", playback.DroppedFrames,
		"buffer_target_seconds", playback.BufferTargetSeconds,
		"previous_buffer_target_seconds", playback.PreviousBufferTargetSeconds,
		"automatic", playback.Automatic,
	)
}

func (s *Server) handleSupportExport(w http.ResponseWriter, r *http.Request) {
	var request supportExportRequest
	if err := decodeSupportJSON(w, r, &request, true); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_support_request")
		return
	}
	normalizeBrowserDiagnostics(&request.Client)
	s.log.Info("support report requested", browserLogGroup(request.Client))

	diagnostics := s.collectDiagnostics(r.Context())
	logs := []struct {
		Name string
		Data []byte
	}{}
	if s.supportLogs != nil {
		snapshot, err := s.supportLogs.Snapshot()
		if err != nil {
			s.log.Error("reading diagnostic history for support export failed", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "support_export_failed")
			return
		}
		for _, file := range snapshot {
			logs = append(logs, struct {
				Name string
				Data []byte
			}{Name: file.Name, Data: file.Data})
		}
	}

	diagnosticJSON, err := json.MarshalIndent(struct {
		Schema       int                 `json:"schema"`
		Theia        diagnosticsResponse `json:"theia"`
		ExportClient browserDiagnostics  `json:"export_client"`
	}{Schema: 1, Theia: diagnostics, ExportClient: request.Client}, "", "  ")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "support_export_failed")
		return
	}

	var archive bytes.Buffer
	zipper := zip.NewWriter(&archive)
	files := []struct {
		name string
		data []byte
	}{
		{name: "README.txt", data: []byte(s.supportReadme(diagnostics, len(logs)))},
		{name: "diagnostics.json", data: []byte(s.sanitizeSupportText(string(diagnosticJSON)))},
	}
	for _, logFile := range logs {
		files = append(files, struct {
			name string
			data []byte
		}{name: "logs/" + logFile.Name + ".jsonl", data: []byte(s.sanitizeSupportText(string(logFile.Data)))})
	}
	for _, file := range files {
		entry, createErr := zipper.Create(file.name)
		if createErr != nil {
			err = createErr
			break
		}
		if _, writeErr := entry.Write(file.data); writeErr != nil {
			err = writeErr
			break
		}
	}
	if closeErr := zipper.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		s.log.Error("building support archive failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "support_export_failed")
		return
	}

	name := "theia-support-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(archive.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive.Bytes())
}

func decodeSupportJSON(w http.ResponseWriter, r *http.Request, target any, emptyAllowed bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, supportPayloadLimit)
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(target)
	if err == io.EOF && emptyAllowed {
		return nil
	}
	if err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("diagnostic payload contains trailing data")
	}
	return nil
}

func normalizeBrowserDiagnostics(client *browserDiagnostics) {
	client.UserAgent = boundedText(client.UserAgent, 500)
	client.Platform = boundedText(client.Platform, 100)
	client.Language = boundedText(client.Language, 35)
	client.ConnectionType = boundedText(client.ConnectionType, 30)
	client.HardwareConcurrency = boundedInt(client.HardwareConcurrency, 0, 1024)
	client.ScreenWidth = boundedInt(client.ScreenWidth, 0, 32768)
	client.ScreenHeight = boundedInt(client.ScreenHeight, 0, 32768)
	client.DeviceMemoryGB = boundedFloat(client.DeviceMemoryGB, 0, 4096)
	client.PixelRatio = boundedFloat(client.PixelRatio, 0, 16)
	client.DownlinkMbps = boundedFloat(client.DownlinkMbps, 0, 100000)
}

func normalizePlaybackDiagnostic(playback *playbackDiagnostic) {
	playback.SessionID = boundedText(playback.SessionID, 80)
	playback.ItemKind = boundedText(playback.ItemKind, 20)
	playback.Mode = boundedText(playback.Mode, 30)
	playback.ReasonCode = boundedText(playback.ReasonCode, 80)
	playback.VideoCodec = boundedText(playback.VideoCodec, 40)
	playback.AudioCodec = boundedText(playback.AudioCodec, 40)
	playback.Encoder = boundedText(playback.Encoder, 80)
	playback.EncoderKind = boundedText(playback.EncoderKind, 30)
	playback.HardwareDecoder = boundedText(playback.HardwareDecoder, 80)
	playback.SourceHeight = boundedInt(playback.SourceHeight, 0, 32768)
	playback.OutputHeight = boundedInt(playback.OutputHeight, 0, 32768)
	playback.ReadyState = boundedInt(playback.ReadyState, 0, 4)
	playback.FrameRate = boundedFloat(playback.FrameRate, 0, 1000)
	playback.PositionSeconds = boundedFloat(playback.PositionSeconds, 0, 60*60*24*30)
	playback.BufferedSeconds = boundedFloat(playback.BufferedSeconds, 0, 60*60*24)
	playback.BufferTargetSeconds = boundedFloat(playback.BufferTargetSeconds, 0, 600)
	playback.PreviousBufferTargetSeconds = boundedFloat(playback.PreviousBufferTargetSeconds, 0, 600)
}

func boundedText(value string, maximum int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maximum {
		return string(runes[:maximum])
	}
	return value
}

func boundedInt(value, minimum, maximum int) int {
	if value < minimum || value > maximum {
		return 0
	}
	return value
}

func boundedFloat(value, minimum, maximum float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < minimum || value > maximum {
		return 0
	}
	return value
}

func (s *Server) supportReadme(diagnostics diagnosticsResponse, logFiles int) string {
	encoderNames := make([]string, 0, len(diagnostics.FFmpeg.Encoders))
	for _, encoder := range diagnostics.FFmpeg.Encoders {
		encoderNames = append(encoderNames, fmt.Sprintf("%s (%s)", encoder.Name, encoder.Kind))
	}
	if len(encoderNames) == 0 {
		encoderNames = append(encoderNames, "none measured")
	}
	watching := make([]string, 0, len(s.libraryRoots()))
	for index, root := range s.libraryRoots() {
		state := "unavailable"
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			state = "available"
		}
		watching = append(watching, fmt.Sprintf("  - <LIBRARY_%d>: %s", index+1, state))
	}
	if len(watching) == 0 {
		watching = append(watching, "  - none configured")
	}
	lastScan := "none in this process"
	if scan := diagnostics.Library.LastScan; scan != nil {
		lastScan = fmt.Sprintf("%s; %.3fs; found=%d added=%d updated=%d removed=%d metadata_errors=%d problems=%d",
			scan.StartedAt.UTC().Format(time.RFC3339), scan.Seconds, scan.Found, scan.Added,
			scan.Updated, scan.Removed, scan.MetadataErrors, len(scan.Problems))
	}
	return fmt.Sprintf(`THEIA SUPPORT REPORT

Generated: %s
Version: %s
Platform: %s/%s
Go runtime: %s
Logical CPUs: %d
Process uptime: %d seconds
Process memory: allocated=%d bytes, reserved=%d bytes, goroutines=%d

FFmpeg
  Supported on platform: %t
  Present and verified: %t
  Capabilities measured: %t
  Encoders: %s
  Hardware decoder: %s

Library
  Films: %d
  Series: %d
  Episodes: %d
  Scan running: %t
  Last scan: %s
  Watched folders:
%s

Diagnostic history
  Log files included: %d
  Raw retained size before compression: %d bytes

Privacy
  This archive was generated locally. Theia did not upload it.
  Account home, data and library paths were replaced with placeholders.
  API keys, tokens, passwords and private configuration values were redacted.
  Stable media and file IDs remain so events can be correlated without filenames.

diagnostics.json is the structured machine snapshot.
logs/*.jsonl contains chronological server and browser events.
`, diagnostics.GeneratedAt.Format(time.RFC3339), s.version,
		diagnostics.System.OS, diagnostics.System.Arch, diagnostics.System.GoVersion,
		diagnostics.System.LogicalCPUs, diagnostics.Process.UptimeSeconds,
		diagnostics.Process.MemoryAllocBytes, diagnostics.Process.MemorySysBytes,
		diagnostics.Process.Goroutines, diagnostics.FFmpeg.Supported,
		diagnostics.FFmpeg.Present, diagnostics.FFmpeg.Probed,
		strings.Join(encoderNames, ", "), valueOr(diagnostics.FFmpeg.HardwareDecoder, "none"),
		diagnostics.Library.Films, diagnostics.Library.Series, diagnostics.Library.Episodes,
		diagnostics.Library.Scanning, lastScan, strings.Join(watching, "\n"), logFiles,
		diagnostics.Logs.Bytes)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var (
	jsonSecretPattern = regexp.MustCompile(`(?i)("(?:key|api_key|tmdb_api_key|token|access_token|refresh_token|secret|password|authorization|private_key|client_config)"\s*:\s*)"[^"]*"`)
	textSecretPattern = regexp.MustCompile(`(?i)\b((?:api[_-]?)?key|token|secret|password|authorization|private[_-]?key|client[_-]?config)=([^\s,]+)`)
	bearerPattern     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	jwtPattern        = regexp.MustCompile(`\b[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{8,}\b`)
)

func (s *Server) sanitizeSupportText(value string) string {
	type replacement struct{ old, new string }
	replacements := []replacement{}
	for index, root := range s.libraryRoots() {
		if root != "" {
			replacements = append(replacements, replacement{root, fmt.Sprintf("<LIBRARY_%d>", index+1)})
		}
	}
	if dataDir := s.cfg.Dir(); dataDir != "" {
		replacements = append(replacements, replacement{dataDir, "<DATA_DIR>"})
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		replacements = append(replacements, replacement{home, "<HOME>"})
	}
	sort.SliceStable(replacements, func(i, j int) bool { return len(replacements[i].old) > len(replacements[j].old) })
	for _, item := range replacements {
		variants := []string{
			item.old,
			filepath.ToSlash(item.old),
			strings.ReplaceAll(item.old, `\`, `\\`),
		}
		for _, old := range variants {
			if old == "" {
				continue
			}
			// Persistent records are JSON Lines. Once an absolute private root
			// appears inside a JSON string, hide the remainder of that path too:
			// replacing only the root would still publish the media filename.
			pathPattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(old) + `[^"\r\n]*`)
			value = pathPattern.ReplaceAllString(value, item.new)
		}
	}
	value = jsonSecretPattern.ReplaceAllString(value, `${1}"[REDACTED]"`)
	value = textSecretPattern.ReplaceAllString(value, `${1}=[REDACTED]`)
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = jwtPattern.ReplaceAllString(value, "[REDACTED_TOKEN]")
	return value
}
