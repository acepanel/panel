package stalwart

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
)

var queueProps = []string{"createdAt", "nextRetry", "returnPath", "recipients", "size"}

func toQueueMessage(m swQueuedMessage) QueueMessage {
	recipients := lo.MapToSlice(m.Recipients, func(address string, rcpt swRecipient) QueueRecipient {
		status := strings.ToLower(rcpt.Status.Type)
		switch rcpt.Status.Type {
		case "TemporaryFailure":
			status = "temporary_failure"
		case "PermanentFailure":
			status = "permanent_failure"
		}
		var details []string
		if rcpt.Status.ErrorType != "" {
			details = append(details, rcpt.Status.ErrorType)
		}
		if msg := deref(rcpt.Status.ErrorMessage); msg != "" {
			details = append(details, msg)
		}
		if msg := deref(rcpt.Status.ResponseMessage); msg != "" {
			details = append(details, msg)
		}
		return QueueRecipient{
			Address:    address,
			Status:     status,
			Error:      strings.Join(details, ": "),
			RetryCount: rcpt.RetryCount,
			RetryDue:   rcpt.RetryDue,
		}
	})
	slices.SortFunc(recipients, func(a, b QueueRecipient) int { return strings.Compare(a.Address, b.Address) })
	return QueueMessage{
		ID:         m.ID,
		CreatedAt:  m.CreatedAt,
		NextRetry:  m.NextRetry,
		ReturnPath: m.ReturnPath,
		Size:       m.Size,
		Recipients: recipients,
	}
}

// QueueList 待投递队列
func (s *App) QueueList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[QueueList](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	filter := map[string]any{}
	if req.Keyword != "" {
		filter["text"] = req.Keyword
	}
	messages, total, err := queryGet[swQueuedMessage](r.Context(), c, "QueuedMessage", filter, (req.Page-1)*req.Limit, req.Limit, queueProps)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail queue viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Uint64("page", uint64(req.Page)), slog.String("keyword", req.Keyword))
	service.Success(w, chix.M{
		"total": total,
		"items": lo.Map(messages, func(m swQueuedMessage, _ int) QueueMessage { return toQueueMessage(m) }),
	})
}

// QueueRetry 立即重投，永久失败的收件人要先改回待投递状态
func (s *App) QueueRetry(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[QueueID](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	message, err := getOne[swQueuedMessage](r.Context(), c, "QueuedMessage", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	reset := map[string]any{}
	for address, rcpt := range message.Recipients {
		if rcpt.Status.Type == "PermanentFailure" {
			reset["recipients/"+address+"/status"] = map[string]any{"@type": "Scheduled"}
		}
	}
	if len(reset) > 0 {
		if err = c.update(r.Context(), "QueuedMessage", req.ID, reset); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	if err = c.update(r.Context(), "QueuedMessage", req.ID, map[string]any{"nextRetry": time.Now().UTC().Format(time.RFC3339)}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail queue message retried"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("message_id", req.ID), slog.String("return_path", message.ReturnPath), slog.Any("recipients", lo.Keys(message.Recipients)))
	service.Success(w, nil)
}

// QueueDelete 从队列中删除邮件，不会给发件人退信
func (s *App) QueueDelete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[QueueID](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	message, err := getOne[swQueuedMessage](r.Context(), c, "QueuedMessage", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	if err = c.destroy(r.Context(), "QueuedMessage", []string{req.ID}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("mail queue message deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("message_id", req.ID), slog.String("return_path", message.ReturnPath), slog.Any("recipients", lo.Keys(message.Recipients)))
	service.Success(w, nil)
}
