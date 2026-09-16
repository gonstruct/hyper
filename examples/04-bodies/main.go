// Everything that is not a JSON body, and the other ways to read a response.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gonstruct/hyper"
)

type Token struct {
	AccessToken string `json:"access_token"`
}

type Upload struct {
	ID        string `json:"id"`
	UploadURL string `json:"upload_url"`
}

func main() {
	ctx := context.Background()
	studio := hyper.New(ctx, hyper.Base("https://api.tjp.com"), hyper.BearerToken("sk_..."))

	// application/x-www-form-urlencoded
	token, err := hyper.Post(ctx, "https://oauth.example.com/token", hyper.Form{"grant_type": "client_credentials", "client_id": "abc"}).JSON[Token]()
	if err != nil {
		panic(err)
	}
	fmt.Println(token.AccessToken)

	// multipart/form-data with a file
	source, _ := os.Open("source.png")
	defer source.Close()
	upload, err := studio.Post("/v1/uploads", hyper.Multipart{
		Fields: hyper.Form{"purpose": "reference"},
		Files:  []hyper.File{{Field: "file", Name: "source.png", Body: source}},
	}).JSON[Upload]()
	if err != nil {
		panic(err)
	}

	// Raw bytes to a presigned PUT. No base, no token: the signature is in
	// the URL, so this goes through the package, not the client.
	data, _ := os.ReadFile("source.png")
	if err := hyper.Put(ctx, upload.UploadURL, hyper.Bytes(data), hyper.ContentType("image/png")).Err(); err != nil {
		panic(err)
	}

	// A stream in: the bytes are not in memory. Size becomes Content-Length.
	video, _ := os.Open("source.mp4")
	info, _ := video.Stat()
	if err := hyper.Put(ctx, upload.UploadURL, hyper.Stream(video, info.Size()), hyper.ContentType("video/mp4")).Err(); err != nil {
		panic(err)
	}

	// A stream out: written straight to the file, never buffered.
	target, _ := os.Create("output.mp4")
	defer target.Close()
	if err := hyper.Get(ctx, "https://cdn.example.com/signed/output.mp4", hyper.Sink(target)).Err(); err != nil {
		panic(err)
	}

	// Bytes, text, and the response as a whole.
	png, err := studio.Get("https://cdn.example.com/signed/output.png").Bytes()
	robots, err := studio.Get("https://cdn.example.com/robots.txt").Text()
	response := studio.Get("/v1/models")
	fmt.Println(len(png), len(robots), err, response.Status(), response.Header("Content-Type"))
}
