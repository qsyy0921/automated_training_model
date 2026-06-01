package qqbot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type OneBotEvent struct {
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type"`
	SubType     string          `json:"sub_type,omitempty"`
	MessageID   json.RawMessage `json:"message_id,omitempty"`
	UserID      json.RawMessage `json:"user_id,omitempty"`
	GroupID     json.RawMessage `json:"group_id,omitempty"`
	SelfID      json.RawMessage `json:"self_id,omitempty"`
	Sender      OneBotSender    `json:"sender,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	RawMessage  string          `json:"raw_message,omitempty"`
	Time        int64           `json:"time,omitempty"`
}

type OneBotSender struct {
	UserID   json.RawMessage `json:"user_id,omitempty"`
	Nickname string          `json:"nickname,omitempty"`
	Card     string          `json:"card,omitempty"`
}

type OneBotSegment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type OneBotReply struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
	Echo   string         `json:"echo,omitempty"`
}

func NormalizeEvent(event OneBotEvent, accountID string) channel.InboundMessage {
	if accountID == "" {
		accountID = "default"
	}
	messageID := rawID(event.MessageID)
	if messageID == "" {
		messageID = fmt.Sprintf("onebot_%d", time.Now().UnixNano())
	}
	senderID := rawID(event.UserID)
	if senderID == "" {
		senderID = rawID(event.Sender.UserID)
	}
	senderName := event.Sender.Nickname
	if senderName == "" {
		senderName = event.Sender.Card
	}
	text, attachments, mentioned := parseMessage(event)
	peerKind := channel.PeerKindDirect
	peerID := senderID
	if event.MessageType == "group" {
		peerKind = channel.PeerKindGroup
		peerID = rawID(event.GroupID)
	}
	receivedAt := time.Now()
	if event.Time > 0 {
		receivedAt = time.Unix(event.Time, 0)
	}
	return channel.InboundMessage{
		ID:        messageID,
		Channel:   channel.KindQQ,
		AccountID: accountID,
		Peer: channel.Peer{
			Channel:   channel.KindQQ,
			AccountID: accountID,
			Kind:      peerKind,
			ID:        peerID,
		},
		SenderID:    senderID,
		SenderName:  senderName,
		Text:        strings.TrimSpace(text),
		Mentioned:   mentioned,
		Attachments: attachments,
		ReceivedAt:  receivedAt,
	}
}

func BuildSendMessage(reply channel.OutboundMessage) OneBotReply {
	params := map[string]any{
		"message": reply.Text,
	}
	if reply.Peer.Kind == channel.PeerKindGroup {
		params["message_type"] = "group"
		params["group_id"] = reply.Peer.ID
	} else {
		params["message_type"] = "private"
		params["user_id"] = reply.Peer.ID
	}
	return OneBotReply{Action: "send_msg", Params: params}
}

func parseMessage(event OneBotEvent) (string, []channel.Attachment, bool) {
	if len(event.Message) == 0 {
		return event.RawMessage, nil, strings.Contains(event.RawMessage, "[CQ:at,")
	}
	var plain string
	if err := json.Unmarshal(event.Message, &plain); err == nil {
		if event.RawMessage != "" {
			return event.RawMessage, nil, strings.Contains(event.RawMessage, "[CQ:at,")
		}
		return plain, nil, strings.Contains(plain, "[CQ:at,")
	}
	var segments []OneBotSegment
	if err := json.Unmarshal(event.Message, &segments); err != nil {
		return event.RawMessage, nil, strings.Contains(event.RawMessage, "[CQ:at,")
	}
	var texts []string
	var attachments []channel.Attachment
	mentioned := false
	for i, segment := range segments {
		switch segment.Type {
		case "text":
			if text, ok := segment.Data["text"].(string); ok {
				texts = append(texts, text)
			}
		case "at":
			mentioned = true
		case "image", "file", "record", "video":
			id := stringValue(segment.Data["file"])
			if id == "" {
				id = fmt.Sprintf("seg_%d", i)
			}
			attachments = append(attachments, channel.Attachment{
				ID:        id,
				Name:      id,
				MediaType: segment.Type,
				SourceURI: stringValue(segment.Data["url"]),
				Status:    channel.AttachmentReceived,
				CreatedAt: time.Now(),
			})
		}
	}
	return strings.Join(texts, ""), attachments, mentioned
}

func rawID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err == nil {
		switch value := anyValue.(type) {
		case float64:
			return strconv.FormatInt(int64(value), 10)
		}
	}
	return strings.Trim(string(raw), `"`)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	default:
		return ""
	}
}
