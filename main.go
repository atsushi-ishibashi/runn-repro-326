package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	port      = flag.String("port", "8080", "port to listen")
	client    = flag.Bool("client", false, "client flag")
	badclient = flag.Bool("badclient", false, "badclient flag")
	before    []byte
)

func main() {
	flag.Parse()
	if *client {
		runClient()
		return
	} else if *badclient {
		runBadClient()
		return
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	//X-Request-ID
	e.Use(middleware.RequestID())
	e.Use(middleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
		if len(before) == 0 {
			before = reqBody
		} else {
			if diff := cmp.Diff(reqBody, before, nil); diff != "" {
				log.Println(diff)
				log.Println("\nbefore-------------------------------------------------")
				log.Println(before)
				log.Println("\nafter-------------------------------------------------")
				log.Println(reqBody)
			}
		}
	}))

	//access log
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			req := c.Request()
			return req.URL.Path == "/healthcheck" ||
				req.Method == "OPTIONS"
		},
		Format: `{` +
			`"level":"info",` +
			`"data":{"remote_ip":"${remote_ip}","method":"${method}","uri":"${uri}","status":${status},"latency":${latency},"latency_human":"${latency_human}","requestId":"${id}"},` +
			`"type":"access",` +
			`"time":"${time_rfc3339}"` +
			"}\n",
	}))

	// cache-control
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-cache, no-store")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")
			return next(c)
		}
	})

	e.POST("/upload", Upload)

	log.Fatalln(e.Start(":" + *port))
}

type UploadInput struct {
	Types         string   `form:"types"`
	documentTypes []string `form:"-"`
}

func (i *UploadInput) Build() {
	i.documentTypes = strings.Split(i.Types, ",")
}

func Upload(c echo.Context) error {
	var param UploadInput
	if err := c.Bind(&param); err != nil {
		log.Println(err)
		return err
	}
	log.Printf("beforeBuild: %v\n", param)
	param.Build()
	log.Printf("afterBuild: %v\n", param)
	form, err := c.MultipartForm()
	if err != nil {
		log.Println(err)
		return err
	}
	log.Printf("form: \n%v\n", form)
	return nil
}

func runClient() {
	httpClient := &http.Client{}
	body, contentType := makeBody()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/upload", body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := httpClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	log.Println(resp.Status)
}

func runBadClient() {
	httpClient := &http.Client{}
	body, contentType := makeBody()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost:8080/upload", body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", contentType)

	body2, _ := makeBody()
	req.Body = io.NopCloser(body2)

	resp, err := httpClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	log.Println(resp.Status)
}

func makeBody() (*bytes.Buffer, string) {
	quoteEscaper := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	fieldName := "files"
	fileName := "dummy.png"
	b, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}

	body := &bytes.Buffer{}

	mw := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			quoteEscaper.Replace(fieldName), quoteEscaper.Replace(filepath.Base(fileName))))
	h.Set("Content-Type", http.DetectContentType(b))

	fw, err := mw.CreatePart(h)
	if err != nil {
		panic(err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(b)); err != nil {
		panic(err)
	}
	if err := mw.Close(); err != nil {
		panic(err)
	}
	return body, mw.FormDataContentType()
}
