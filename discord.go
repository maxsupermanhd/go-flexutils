package goflexutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/maxsupermanhd/lac/v2"
	"github.com/rs/zerolog"
)

type DiscordPoster struct {
	cfg lac.Conf
	q   chan string
	log zerolog.Logger
}

func NewDiscordPoster(log zerolog.Logger, cfg lac.Conf) *DiscordPoster {
	return &DiscordPoster{
		cfg: cfg,
		q:   make(chan string, cfg.GetDInt(64, "buffer")),
		log: log,
	}
}

func (dp *DiscordPoster) Queue(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	select {
	case dp.q <- msg:
	default:
		dp.log.Warn().Str("msg", msg).Msg("message queue full")
	}
}

func (dp *DiscordPoster) Routine(closeChan <-chan struct{}) {
	lastAggregate := ""
	dflusher := time.NewTicker(time.Second * time.Duration(dp.cfg.GetDInt(60, "timer")))
	for {
		webhookUrl, ok := dp.cfg.GetString("url")
		select {
		case <-closeChan:
			if len(lastAggregate) < 1995 {
				dp.discordSendErrorWithContent(webhookUrl, lastAggregate)
			} else {
				dp.discordSendErrorWithFile(webhookUrl, lastAggregate)
			}
		case msg := <-dp.q:
			lastAggregate += msg + "\n"
		case <-dflusher.C:
			if lastAggregate == "" {
				continue
			}
			if !ok {
				dp.log.Println("Errors discord webhook not set!!!")
				return
			}
			if len(lastAggregate) < 1995 {
				dp.discordSendErrorWithContent(webhookUrl, lastAggregate)
			} else {
				dp.discordSendErrorWithFile(webhookUrl, lastAggregate)
			}
			lastAggregate = ""
		}
	}
}

func (dp *DiscordPoster) discordSendErrorWithContent(webhookUrl, content string) {
	payload_json, err := json.Marshal(map[string]interface{}{
		"username": dp.cfg.GetDString("DiscordPoster", "username"),
		"content":  content,
	})
	if err != nil {
		dp.log.Error().Err(err).Msg("marshling webhook json payload")
		return
	}
	req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(payload_json))
	if err != nil {
		dp.log.Error().Err(err).Msg("creating webhook request")
		return
	}
	req.Header.Add("Content-Type", "application/json")
	c := http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		dp.log.Error().Err(err).Msg("sending webhook")
		return
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		defer resp.Body.Close()
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			dp.log.Error().Err(err).Msg("sending webhook")
		}
		dp.log.Error().Str("body", string(responseBody)).Msg("sending webhook")
	}
}

func (dp *DiscordPoster) discordSendErrorWithFile(webhookUrl, content string) {
	payload_json, err := json.Marshal(map[string]interface{}{
		"username": dp.cfg.GetDString("DiscordPoster", "username"),
	})
	if err != nil {
		dp.log.Error().Err(err).Msg("marshling webhook json")
		return
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	p1w, err := w.CreateFormField("payload_json")
	if err != nil {
		dp.log.Error().Err(err).Msg("creating webhook json multipart")
		return
	}
	_, err = p1w.Write(payload_json)
	if err != nil {
		dp.log.Error().Err(err).Msg("writing webhook json multipart")
		return
	}
	p2w, err := w.CreateFormFile("file[0]", "msg.txt")
	if err != nil {
		dp.log.Error().Err(err).Msg("creating webhook content multipart")
		return
	}
	_, err = p2w.Write([]byte(content))
	if err != nil {
		dp.log.Error().Err(err).Msg("writing webhook content multipart")
		return
	}
	w.Close()

	req, err := http.NewRequest("POST", webhookUrl, &b)
	if err != nil {
		dp.log.Error().Err(err).Msg("creating webhook request")
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	c := http.Client{Timeout: 5 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		dp.log.Error().Err(err).Msg("sending webhook request")
		return
	}
	if res.StatusCode != http.StatusOK {
		rspb, err := io.ReadAll(res.Body)
		if err != nil {
			dp.log.Error().Err(err).Msg("reading discord's webhook response")
		}
		dp.log.Error().Int("code", res.StatusCode).Str("response", string(rspb)).Msg("discord returned not 200")
	}
}
