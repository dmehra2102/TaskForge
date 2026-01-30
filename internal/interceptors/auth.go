package interceptors

import (
	"context"
	"strings"

	"github.com/dmehra2102/TaskForge/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var publicMethods = map[string]bool{
	"/grpc.health.v1.Health/Check": true,
	"/grpc.health.v1.Health/Watch": true,
}

func AuthInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Etract Metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Get authorization header
		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		// Parse token
		tokenString := strings.TrimPrefix(authHeader[0], "Bearer ")
		if tokenString == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		// Validate JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, status.Error(codes.Unauthenticated, "invalid token signing method")
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token claims")
		}

		userCtx := &auth.UserContext{
			UserID:   claims["user_id"].(string),
			TenantID: claims["tenant_id"].(string),
			Roles:    extractRoles(claims["roles"]),
		}

		// Add to context
		ctx = auth.ContextWithUserContext(ctx, userCtx)

		return handler(ctx, req)
	}
}

func extractRoles(rolesInterface any) []string {
	if rolesInterface == nil {
		return []string{}
	}

	rolesSlice, ok := rolesInterface.([]any)
	if !ok {
		return []string{}
	}

	roles := make([]string, 0, len(rolesSlice))
	for _, role := range rolesSlice {
		if roleStr, ok := role.(string); ok {
			roles = append(roles, roleStr)
		}
	}

	return roles
}
