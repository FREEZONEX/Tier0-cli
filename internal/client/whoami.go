package client

import (
	"context"
	"errors"
	"net/http"

	"github.com/FREEZONEX/Tier0-cli/internal/apierr"
)

// Whoami uses the current OpenAPI contract and supports older deployments that
// expose only v1. Authentication and authorization failures are never retried.
func (c *Client) Whoami(ctx context.Context, debug bool) (string, error) {
	return c.versionedPost(ctx, "/auth/whoami", debug)
}

// Info supports the current endpoint and older v1-only deployments.
func (c *Client) Info(ctx context.Context, debug bool) (string, error) {
	return c.versionedPost(ctx, "/info", debug)
}

func (c *Client) versionedPost(ctx context.Context, path string, debug bool) (string, error) {
	response, err := c.DoAPI(ctx, "/openapi/v2"+path, http.MethodPost, "{}", debug)
	var apiError *apierr.APIError
	if errors.As(err, &apiError) && apiError.HTTPStatus == http.StatusNotFound {
		return c.DoAPI(ctx, "/openapi/v1"+path, http.MethodPost, "{}", debug)
	}
	return response, err
}
