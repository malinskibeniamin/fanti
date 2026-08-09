package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
)

func TestHandlerOptionsLimitRequestSize(t *testing.T) {
	_, handler := fantiv1connect.NewConversionServiceHandler(
		&Conversions{}, handlerOptions(slog.Default(), 128)...)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := fantiv1connect.NewConversionServiceClient(http.DefaultClient, srv.URL)
	_, err := client.CreateConversion(t.Context(), connect.NewRequest(&fantiv1.CreateConversionRequest{
		Filename: "large.txt",
		Data:     make([]byte, 1024),
	}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("code = %v, want ResourceExhausted", connect.CodeOf(err))
	}
}
