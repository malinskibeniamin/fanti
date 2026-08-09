package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"

	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/convert"
)

// Allow a 64 MiB file plus protobuf framing and request metadata.
const maxRequestBytes = 65 << 20

func handlerOptions(logger *slog.Logger, readMaxBytes int) []connect.HandlerOption {
	return []connect.HandlerOption{
		connect.WithInterceptors(AccessLog(logger)),
		connect.WithReadMaxBytes(readMaxBytes),
	}
}

// NewHandler mounts every fanti.v1 service plus health, wrapped in
// dev-friendly CORS for the Rsbuild dev server.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) (http.Handler, error) {
	engine, err := convert.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("load conversion engine: %w", err)
	}

	if _, err := pool.Exec(context.Background(), `
		UPDATE conversions
		SET state = 'failed', progress_percent = 0,
			error_message = 'Conversion interrupted by a server restart. Run it again.',
			update_time = now()
		WHERE state = 'running'`); err != nil {
		return nil, fmt.Errorf("recover interrupted conversions: %w", err)
	}

	options := handlerOptions(logger, maxRequestBytes)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle(fantiv1connect.NewDictionaryServiceHandler(NewDictionary(pool), options...))
	mux.Handle(fantiv1connect.NewLibraryServiceHandler(NewLibrary(pool), options...))
	mux.Handle(fantiv1connect.NewConversionServiceHandler(NewConversions(pool, engine, logger), options...))
	mux.Handle(fantiv1connect.NewStudyServiceHandler(NewStudy(pool), options...))
	mux.Handle(fantiv1connect.NewTutorServiceHandler(NewTutor(pool), options...))

	c := cors.New(cors.Options{
		// Single-user app: any localhost origin is the dev server.
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	})

	return c.Handler(mux), nil
}
