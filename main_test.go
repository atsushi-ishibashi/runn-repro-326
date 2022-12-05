package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/k1LoW/runn"
)

func TestHoge(t *testing.T) {
	e := InitEcho()
	// req := httptest.NewRequest(echo.GET, "/", nil)
	// rec := httptest.NewRecorder()

	ctx := context.Background()
	ts := httptest.NewServer(e)
	t.Cleanup(func() {
		ts.Close()
	})
	opts := []runn.Option{
		runn.T(t),
		runn.Runner("amreq", ts.URL),
	}
	o, err := runn.Load("book.yml", opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.RunN(ctx); err != nil {
		t.Fatal(err)
	}
}
