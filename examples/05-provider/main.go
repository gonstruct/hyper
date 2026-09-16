// A provider client the way an app writes one: an interface the app depends
// on, a struct around a hyper client, one line per endpoint. This is the tjp
// Studio client, rewritten.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gonstruct/hyper"
)

type Model struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	ContractHash string `json:"contract_hash"`
}

type Upload struct {
	ID        string `json:"id"`
	UploadURL string `json:"upload_url"`
}

type GenerationRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt,omitempty"`
	Options map[string]any `json:"options"`
}

type Generation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Studio interface {
	Models() ([]Model, error)
	Upload(name, mime string, body io.Reader, size int64) (Upload, error)
	Generate(request GenerationRequest, idempotencyKey string) (Generation, error)
	Generation(id string) (Generation, error)
	Download(url string, into io.Writer) error
}

type client struct {
	api hyper.Client // base, token, tracing, retry
	raw hyper.Client // presigned URLs: no base, no token, same tracing, long timeout
}

func NewStudio(ctx context.Context, baseURL, apiKey string, transport http.RoundTripper) Studio {
	return &client{
		api: hyper.New(ctx,
			hyper.Base(baseURL),
			hyper.BearerToken(apiKey),
			hyper.Transport(transport),
			hyper.Timeout(30*time.Second),
			hyper.Retry(3, hyper.Backoff(time.Second, 10*time.Second)),
		),
		raw: hyper.New(ctx, hyper.Transport(transport), hyper.Timeout(5*time.Minute)),
	}
}

func (c *client) Models() ([]Model, error) {
	return c.api.Get("/v1/models").JSON[[]Model]("data")
}

func (c *client) Upload(name, mime string, body io.Reader, size int64) (Upload, error) {
	upload, err := c.api.Post("/v1/uploads", map[string]any{"filename": name, "content_type": mime, "size": size}).JSON[Upload]()
	if err != nil {
		return upload, err
	}
	return upload, c.raw.Put(upload.UploadURL, hyper.Stream(body, size), hyper.ContentType(mime)).Err()
}

func (c *client) Generate(request GenerationRequest, idempotencyKey string) (Generation, error) {
	return c.api.Post("/v1/generations", request, hyper.Header("Idempotency-Key", idempotencyKey)).JSON[Generation]()
}

func (c *client) Generation(id string) (Generation, error) {
	return c.api.Get("/v1/generations/" + id).JSON[Generation]()
}

func (c *client) Download(url string, into io.Writer) error {
	return c.raw.Get(url, hyper.Sink(into)).Err()
}

func main() {
	studio := NewStudio(context.Background(), "https://api.tjp.com", "sk_...", nil)
	models, err := studio.Models()
	if err != nil {
		panic(err)
	}
	fmt.Println(len(models), "models")
}
