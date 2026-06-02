package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/qqbot"
)

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) {
	outbound := qqbot.OutboundStatusFromEnv()
	websocket := qqbot.WebSocketStatusFromEnv()
	writeJSON(w, http.StatusOK, map[string]any{
		"channels": []map[string]any{
			{
				"id":               "qq",
				"name":             "QQ / NapCat OneBot",
				"status":           "adapter-ready",
				"runtime":          "agent-runtime",
				"inbound_endpoint": "/api/channels/qq/onebot",
				"test_endpoint":    "/api/channels/qq/test-message",
				"outbound_enabled": outbound.Enabled,
				"websocket":        websocket,
			},
		},
	})
}

func (s *Server) qqStatus(w http.ResponseWriter, r *http.Request) {
	outbound := qqbot.OutboundStatusFromEnv()
	websocket := qqbot.WebSocketStatusFromEnv()
	writeJSON(w, http.StatusOK, map[string]any{
		"channel":          "qq",
		"adapter":          "napcat-onebot",
		"runtime":          "ready",
		"inbound_endpoint": "/api/channels/qq/onebot",
		"test_endpoint":    "/api/channels/qq/test-message",
		"outbound":         outbound,
		"websocket":        websocket,
		"supported_commands": []string{
			"/bot-ping",
			"/bot-me",
			"/bot-status",
			"/bot-runs",
			"/bot-run dry [dataset_id]",
		},
	})
}

func (s *Server) qqTestMessage(w http.ResponseWriter, r *http.Request) {
	var msg channel.InboundMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fillQQDefaults(&msg)
	reply, err := s.runtime.HandleChannelMessage(r.Context(), msg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reply": reply})
}

func (s *Server) runtimeStreamMessage(w http.ResponseWriter, r *http.Request) {
	var msg channel.InboundMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fillQQDefaults(&msg)
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	_, _ = s.runtime.HandleChannelMessageStream(r.Context(), msg, func(event agentruntime.RuntimeStreamEvent) {
		_ = enc.Encode(event)
		if flusher != nil {
			flusher.Flush()
		}
	})
}

func (s *Server) qqOneBotEvent(w http.ResponseWriter, r *http.Request) {
	var event qqbot.OneBotEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	msg := qqbot.NormalizeEvent(event, "default")
	reply, err := s.runtime.HandleChannelMessage(r.Context(), msg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	onebotReply := qqbot.BuildSendMessage(reply)
	outbound := qqbot.OutboundStatusFromEnv()
	outboundStatus := map[string]any{"enabled": outbound.Enabled, "sent": false}
	if outbound.Enabled {
		if err := qqbot.NewHTTPOutboundSender(qqbot.OutboundConfigFromEnv()).Send(r.Context(), onebotReply); err != nil {
			outboundStatus["error"] = err.Error()
		} else {
			outboundStatus["sent"] = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reply":        reply,
		"onebot_reply": onebotReply,
		"outbound":     outboundStatus,
	})
}

func fillQQDefaults(msg *channel.InboundMessage) {
	if msg.Channel == "" {
		msg.Channel = channel.KindQQ
	}
	if msg.AccountID == "" {
		msg.AccountID = "default"
	}
	if msg.Peer.Channel == "" {
		msg.Peer.Channel = channel.KindQQ
	}
	if msg.Peer.AccountID == "" {
		msg.Peer.AccountID = msg.AccountID
	}
	if msg.Peer.Kind == "" {
		msg.Peer.Kind = channel.PeerKindDirect
	}
}
