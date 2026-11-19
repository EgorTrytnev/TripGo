// Пакет push отправляет команды в Push Service.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Client отправляет HTTP-запросы в Push Service.
type Client struct {
	baseURL string
	client  *http.Client
}

// New создаёт клиент Push Service с отдельным таймаутом.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, client: &http.Client{Timeout: timeout}}
}

// SendPositionRequest просит устройство водителя прислать текущую координату.
func (c *Client) SendPositionRequest(ctx context.Context, driverID, tripID uuid.UUID) error {
	ctx, span := otel.Tracer("tripgo.push").Start(ctx, "send position request")
	defer span.End()
	body, err := json.Marshal(map[string]any{
		"request_id":   uuid.NewString(),
		"recipient_id": driverID,
		"kind":         "REQUEST_POSITION",
		"data":         map[string]string{"trip_id": tripID.String()},
	})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "push request failed")
		return fmt.Errorf("push request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("read push response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		span.SetStatus(codes.Error, "push service returned an error")
		return fmt.Errorf("push service returned status %d", resp.StatusCode)
	}
	return nil
}
