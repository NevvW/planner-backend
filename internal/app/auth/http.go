package appauth

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pbv1 "auth_service/pkg/api/proto"
)

//go:embed swagger/v1.swagger.json
var embeddedSwagger []byte

type swaggerDoc struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func RunRest(ctx context.Context, log *zap.Logger, httpAddr string, grpcAddr string, profileHandler http.Handler, todoHandler http.Handler, metricsHandler http.Handler) (*http.Server, error) {
	allowedOrigins := loadAllowedOriginsFromEnv()
	defaultAllowedMethods := loadAllowedMethodsFromEnv()
	defaultAllowedHeaders := loadAllowedHeadersFromEnv()
	jsonMarshaler := authJSONMarshaler{
		JSONPb: runtime.JSONPb{
			MarshalOptions:   protojson.MarshalOptions{UseProtoNames: false},
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		},
	}

	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &jsonMarshaler),
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			md := metadata.Pairs(
				"x-client-ip", extractIP(r),
				"x-user-agent", r.UserAgent(),
			)

			if c, err := r.Cookie("refresh_token"); err == nil {
				md.Append("x-refresh-token", c.Value)
			}

			return md
		}),
		runtime.WithForwardResponseOption(func(ctx context.Context, w http.ResponseWriter, resp proto.Message) error {
			md, ok := runtime.ServerMetadataFromContext(ctx)
			if !ok {
				return nil
			}

			for _, c := range md.HeaderMD.Get("set-cookie") {
				w.Header().Add("Set-Cookie", c)
			}

			b, err := marshalAuthResponseForLog(resp)
			if err != nil {
				log.Warn("http rest response marshal failed", zap.Error(err), zap.String("proto_type", string(resp.ProtoReflect().Descriptor().FullName())))
				return nil
			}
			log.Info("http rest response", zap.String("proto_type", string(resp.ProtoReflect().Descriptor().FullName())), zap.ByteString("payload", b))

			return nil
		}),
	)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if err := pbv1.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, grpcAddr, dialOpts); err != nil {
		return nil, err
	}

	routeMethods, err := loadAllowedMethodsFromEmbeddedSwagger()
	if err != nil {
		return nil, fmt.Errorf("load swagger routes: %w", err)
	}

	rootMux := http.NewServeMux()
	rootMux.Handle("/v1/user/", profileHandler)
	rootMux.Handle("/v1/todo/", todoHandler)
	rootMux.Handle("/v1/metrics", metricsHandler)
	rootMux.Handle("/", mux)
	handler := optionsMiddleware(log, routeMethods, allowedOrigins, defaultAllowedMethods, defaultAllowedHeaders, httpLoggingMiddleware(log, rootMux))

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http serve stopped", zap.Error(err))
		}
	}()

	return srv, nil
}

type authJSONMarshaler struct {
	runtime.JSONPb
}

type authResponseJSON struct {
	AccessToken string  `json:"accessToken"`
	UserID      string  `json:"userId"`
	Login       string  `json:"login"`
	Email       string  `json:"email"`
	ProfileName *string `json:"profileName,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
}

func (m authJSONMarshaler) Marshal(v interface{}) ([]byte, error) {
	switch resp := v.(type) {
	case *pbv1.RegisterResponse:
		return json.Marshal(authResponseJSON{
			AccessToken: resp.AccessToken,
			UserID:      resp.UserId,
			Login:       resp.Login,
			Email:       resp.Email,
			ProfileName: resp.ProfileName,
			AvatarURL:   resp.AvatarUrl,
		})
	case *pbv1.LoginResponse:
		return json.Marshal(authResponseJSON{
			AccessToken: resp.AccessToken,
			UserID:      resp.UserId,
			Login:       resp.Login,
			Email:       resp.Email,
			ProfileName: resp.ProfileName,
			AvatarURL:   resp.AvatarUrl,
		})
	default:
		return m.JSONPb.Marshal(v)
	}
}

func marshalAuthResponseForLog(resp proto.Message) ([]byte, error) {
	return authJSONMarshaler{
		JSONPb: runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{UseProtoNames: false},
		},
	}.Marshal(resp)
}

func loadAllowedMethodsFromEnv() []string {
	allowedMethodsEnv := strings.TrimSpace(os.Getenv("CORS_ALLOWED_METHODS"))
	if allowedMethodsEnv == "" {
		return []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	}

	methodSet := map[string]struct{}{}
	for _, rawMethod := range strings.Split(allowedMethodsEnv, ",") {
		method := strings.ToUpper(strings.TrimSpace(rawMethod))
		if method == "" {
			continue
		}
		methodSet[method] = struct{}{}
	}
	methodSet[http.MethodOptions] = struct{}{}

	methods := make([]string, 0, len(methodSet))
	for method := range methodSet {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func loadAllowedHeadersFromEnv() string {
	allowedHeadersEnv := strings.TrimSpace(os.Getenv("CORS_ALLOWED_HEADERS"))
	if allowedHeadersEnv == "" {
		return "Authorization, Content-Type"
	}
	return allowedHeadersEnv
}

func loadAllowedMethodsFromEmbeddedSwagger() (map[string][]string, error) {
	var doc swaggerDoc
	if err := json.Unmarshal(embeddedSwagger, &doc); err != nil {
		return nil, err
	}

	result := make(map[string][]string, len(doc.Paths))

	for route, ops := range doc.Paths {
		methodSet := make(map[string]struct{})

		for method := range ops {
			m := strings.ToUpper(strings.TrimSpace(method))
			switch m {
			case http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodHead:
				methodSet[m] = struct{}{}
			}
		}

		methodSet[http.MethodOptions] = struct{}{}

		methods := make([]string, 0, len(methodSet))
		for m := range methodSet {
			methods = append(methods, m)
		}
		sort.Strings(methods)

		result[route] = methods
	}

	return result, nil
}

func loadAllowedOriginsFromEnv() map[string]struct{} {
	allowedOriginsEnv := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if allowedOriginsEnv == "" {
		return map[string]struct{}{}
	}

	allowedOrigins := make(map[string]struct{})
	for _, rawOrigin := range strings.Split(allowedOriginsEnv, ",") {
		origin := strings.TrimSpace(rawOrigin)
		if origin == "" {
			continue
		}
		allowedOrigins[origin] = struct{}{}
	}

	return allowedOrigins
}

func optionsMiddleware(log *zap.Logger, routes map[string][]string, allowedOrigins map[string]struct{}, defaultAllowedMethods []string, defaultAllowedHeaders string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if _, ok := allowedOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Add("Vary", "Access-Control-Request-Method")
		}

		methods := resolveAllowedMethodsForPath(r.URL.Path, routes, defaultAllowedMethods)
		w.Header().Set("Allow", strings.Join(methods, ", "))
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

		reqHeaders := r.Header.Get("Access-Control-Request-Headers")
		if reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", defaultAllowedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveAllowedMethodsForPath(path string, routes map[string][]string, defaultAllowedMethods []string) []string {
	if methods, ok := routes[path]; ok {
		return methods
	}

	if strings.HasPrefix(path, "/v1/user/me/settings") {
		return []string{http.MethodGet, http.MethodOptions, http.MethodPatch}
	}
	if strings.HasPrefix(path, "/v1/user/") {
		return []string{http.MethodGet, http.MethodOptions, http.MethodPatch}
	}
	if strings.HasPrefix(path, "/v1/todo/") {
		return []string{http.MethodGet, http.MethodOptions, http.MethodPost}
	}
	if path == "/v1/metrics" {
		return []string{http.MethodGet, http.MethodOptions}
	}

	return defaultAllowedMethods
}

func httpLoggingMiddleware(log *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info("http request",
			zap.String("method", r.Method),
			zap.String("swaggerPath", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Duration("duration", time.Since(start)),
		)
	})
}
func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
