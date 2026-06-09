// Package grpcserver реализует gRPC-сервер сервиса сокращения URL.
// Является фасадом к тому же service.Shortener, что используют HTTP-хендлеры.
package grpcserver

import (
	"context"
	"net/url"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/gearwheels/go_url_shortener/internal/config"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	"github.com/gearwheels/go_url_shortener/internal/service"
	pb "github.com/gearwheels/go_url_shortener/proto"
	"github.com/google/uuid"
)

// ShortenerServer реализует pb.ShortenerServiceServer.
type ShortenerServer struct {
	pb.UnimplementedShortenerServiceServer
}

// ShortenURL соответствует POST /api/shorten.
func (s *ShortenerServer) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	userID, _ := logrequest.GetUserID(ctx)
	shortCode, _, err := service.Shortener.ShortenURL(ctx, req.GetUrl(), userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "shorten: %v", err)
	}
	result, err := url.JoinPath(config.AppConfig.BaseURL, shortCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build url: %v", err)
	}
	return &pb.URLShortenResponse{Result: result}, nil
}

// ExpandURL соответствует GET /{id}.
func (s *ShortenerServer) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	originalURL, deleted, err := service.Shortener.GetOriginalURL(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}
	if deleted {
		return nil, status.Error(codes.NotFound, "URL deleted")
	}
	return &pb.URLExpandResponse{Result: originalURL}, nil
}

// ListUserURLs соответствует GET /api/user/urls.
func (s *ShortenerServer) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, err := logrequest.GetUserID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	urls, err := service.Shortener.GetAllShortenerURL(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get urls: %v", err)
	}
	data := make([]*pb.URLData, 0, len(urls))
	for _, u := range urls {
		shortURL, _ := url.JoinPath(config.AppConfig.BaseURL, u.ShortURL)
		data = append(data, &pb.URLData{ShortUrl: shortURL, OriginalUrl: u.URL})
	}
	return &pb.UserURLsResponse{Url: data}, nil
}

// AuthInterceptor — unary-перехватчик для аутентификации через метаданные.
// Ожидает заголовок "authorization" с форматом "userID:hmac-signature" —
// тот же формат, что и HMAC-cookie в HTTP.
// При отсутствии или невалидном токене генерирует новый userID (анонимный пользователь).
func AuthInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	userID := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			if uid, err := logrequest.ParseAuthCookie(vals[0]); err == nil {
				userID = uid
			}
		}
	}
	if userID == "" {
		userID = uuid.New().String()
	}
	ctx = logrequest.ContextWithUserID(ctx, userID)
	return handler(ctx, req)
}
