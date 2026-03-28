package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
)

type EngineClient struct {
	baseURL     string
	internalKey string
	client      *http.Client
}

func NewEngineClient(baseURL, internalKey string) *EngineClient {
	return &EngineClient{
		baseURL:     baseURL,
		internalKey: internalKey,
		client:      &http.Client{Timeout: 6 * time.Minute},
	}
}

func (c *EngineClient) RunTask(ctx context.Context, req *models.EngineRequest) (*models.EngineResponse, error) {
	return c.post(ctx, "/v1/run_task", req)
}

func (c *EngineClient) Route(ctx context.Context, inputText, orgSoul string) (string, string, error) {
	body := map[string]string{"input_text": inputText, "org_soul": orgSoul}
	var result struct {
		RoleSlug  string `json:"role_slug"`
		Reasoning string `json:"reasoning"`
	}
	if err := c.postDecode(ctx, "/v1/route", body, &result); err != nil {
		return "chief-of-staff", "fallback", err
	}
	return result.RoleSlug, result.Reasoning, nil
}

func (c *EngineClient) Orchestrate(ctx context.Context, objective, orgSoul, orgID, memberID string) (map[string]interface{}, error) {
	body := map[string]string{"objective": objective, "org_soul": orgSoul, "org_id": orgID, "member_id": memberID}
	var result map[string]interface{}
	err := c.postDecode(ctx, "/v1/orchestrate", body, &result)
	return result, err
}

func (c *EngineClient) post(ctx context.Context, path string, body interface{}) (*models.EngineResponse, error) {
	var resp models.EngineResponse
	if err := c.postDecode(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *EngineClient) postDecode(ctx context.Context, path string, body interface{}, target interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.internalKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("engine error %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
	}
	return json.Unmarshal(data, target)
}
