// An API client the way an app writes one: an interface the app depends on,
// a struct around a hyper client, one line per endpoint.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gonstruct/hyper"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Upload struct {
	ID        string `json:"id"`
	UploadURL string `json:"upload_url"`
}

type NewTask struct {
	Project string `json:"project"`
	Title   string `json:"title"`
}

type Task struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Tasks interface {
	Projects() ([]Project, error)
	Upload(name, mime string, body io.Reader, size int64) (Upload, error)
	Create(task NewTask, idempotencyKey string) (Task, error)
	Task(id string) (Task, error)
	Export(id string, into io.Writer) error
}

type client struct {
	api hyper.Client // base, token, tracing, retry
	raw hyper.Client // presigned URLs: no base, no token, same tracing, long timeout
}

func NewTasks(ctx context.Context, baseURL, token string, transport http.RoundTripper) Tasks {
	return &client{
		api: hyper.New(ctx,
			hyper.Base(baseURL),
			hyper.BearerToken(token),
			hyper.Transport(transport),
			hyper.Timeout(30*time.Second),
			hyper.Retry(3, hyper.Backoff(time.Second, 10*time.Second)),
		),
		raw: hyper.New(ctx, hyper.Transport(transport), hyper.Timeout(5*time.Minute)),
	}
}

func (c *client) Projects() ([]Project, error) {
	return c.api.Get("/projects").JSON[[]Project]("data")
}

func (c *client) Upload(name, mime string, body io.Reader, size int64) (Upload, error) {
	upload, err := c.api.Post("/files", map[string]any{"filename": name, "content_type": mime, "size": size}).JSON[Upload]()
	if err != nil {
		return upload, err
	}
	return upload, c.raw.Put(upload.UploadURL, hyper.Stream(body, size), hyper.ContentType(mime)).Err()
}

func (c *client) Create(task NewTask, idempotencyKey string) (Task, error) {
	return c.api.Post("/tasks", task, hyper.Header("Idempotency-Key", idempotencyKey)).JSON[Task]()
}

func (c *client) Task(id string) (Task, error) {
	return c.api.Get("/tasks/" + id).JSON[Task]()
}

func (c *client) Export(id string, into io.Writer) error {
	return c.api.Get("/tasks/"+id+"/export", hyper.Sink(into)).Err()
}

func main() {
	tasks := NewTasks(context.Background(), "https://api.example.com", "token", nil)
	projects, err := tasks.Projects()
	if err != nil {
		panic(err)
	}
	fmt.Println(len(projects), "projects")
}
