package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/qqbot"
)

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"channels": []map[string]any{
			{
				"id":               "qq",
				"name":             "QQ / NapCat OneBot",
				"status":           "adapter-ready",
				"runtime":          "agent-runtime",
				"inbound_endpoint": "/api/channels/qq/onebot",
				"test_endpoint":    "/api/channels/qq/test-message",
			},
		},
	})
}

func (s *Server) qqStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"channel":          "qq",
		"adapter":          "napcat-onebot",
		"runtime":          "ready",
		"inbound_endpoint": "/api/channels/qq/onebot",
		"test_endpoint":    "/api/channels/qq/test-message",
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
	writeJSON(w, http.StatusOK, map[string]any{
		"reply":        reply,
		"onebot_reply": qqbot.BuildSendMessage(reply),
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
