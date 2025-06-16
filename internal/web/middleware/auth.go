package middleware

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"net/http"
	"strconv"
	"strings"
)

const (
	KeyAuthHeader = "Authorization"
	KeyAuthType   = "Bearer"
	KeyAuthCookie = "member"

	KeyJwtUserIdKey       = "sub"
	KeyJwtSessionTokenKey = "tok"

	KeyUserIdKey       = "userId"
	KeySessionTokenKey = "sessionToken"

	KeyJwtSecret = "jwtSecret"
)

var (
	jwtSecret = "no-key"
)

// AuthMiddleware "user" middleware
func AuthMiddleware(params map[string]interface{}, route *config.Route) func(next http.Handler) http.Handler {

	if jwtSecretItem, ok := params[KeyJwtSecret]; ok {
		jwtSecret = jwtSecretItem.(string)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//authType := KeyAuthType
			authToken := r.Header.Get(KeyAuthHeader)

			if authToken == "" {
				if authCookie, err := r.Cookie(KeyAuthCookie); err == nil {
					authToken = authCookie.Value
				}
			}

			fields := strings.Fields(authToken)
			if len(fields) == 2 {
				//authType = fields[0]
				authToken = fields[1]
			}

			var userId common.UserID
			var sessionToken string

			// Attempt to parse userId from JWT
			if authToken != "" {
				parser := new(jwt.Parser)
				parser.SkipClaimsValidation = route.Options.SkipClaimsValidation

				token, err := parser.Parse(authToken, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}
					return []byte(jwtSecret), nil
				})

				if err == nil {
					validClaims := true
					if !parser.SkipClaimsValidation {
						validClaims = token.Claims.Valid() == nil
					}

					// Extract subject from claims
					if claims, ok := token.Claims.(jwt.MapClaims); ok && validClaims {
						if uid, ok := claims[KeyJwtUserIdKey]; ok {
							userId = common.UserID(uid.(float64))
						}
						if token, ok := claims[KeyJwtSessionTokenKey]; ok {
							sessionToken = token.(string)
						}
					}
				}
			}

			requestBody := *r.Context().Value(common.RequestBodyKey).(*map[string]interface{})
			log := r.Context().Value(common.LoggerKey).(*logger.Logger)

			// Force to remove userId and sessionToken from requestBody
			delete(requestBody, KeyUserIdKey)
			delete(requestBody, KeySessionTokenKey)

			if userId != 0 {

				// TODO: Move to middleware
				// Check blacklist by UserId
				//if s.caller.GetService(security.ServiceName).(*security.SecurityService).IsUserBlocked(userId, nil) {
				//	return error.Wrap(error.ErrorCodeForbidden, fmt.Errorf("request blocked"))
				//}

				requestBody[KeyUserIdKey] = userId
				log.AddField(KeyUserIdKey, strconv.FormatUint(uint64(userId), 10))
			}

			if sessionToken != "" {
				requestBody[KeySessionTokenKey] = sessionToken
				log.AddField(KeySessionTokenKey, sessionToken)
			}

			r = r.WithContext(context.WithValue(r.Context(), common.RequestBodyKey, &requestBody))

			next.ServeHTTP(w, r)
		})
	}
}
