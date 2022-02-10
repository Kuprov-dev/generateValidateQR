package generate

import (
	"encoding/json"
	"fmt"
	"generateValidateQR/pkg/conf"
	"generateValidateQR/pkg/db"
	"generateValidateQR/pkg/errors"
	jwtUtils "generateValidateQR/pkg/jwt"
	"generateValidateQR/pkg/models"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt"
)


func ValidateTokenAndRefreshMiddleware(config *conf.Config, userDAO db.UserDAO) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			tokenCreds, err := jwtUtils.GetTokenCredsFromContext(req.Context())
			fmt.Println(tokenCreds)
			if err != nil {
				log.Println("Unathorised error.")
				errors.MakeUnathorisedErrorResponse(&w, err.Error())
				return
			}

			if tokenCreds.AccessToken == "" {
				log.Println("Token is empty.")
				errors.MakeUnathorisedErrorResponse(&w, "An authorization token is required.")
				return
			}
			// TODO это не токен тот который нам нужен
			bearerToken := [2]string{tokenCreds.RefreshToken, tokenCreds.AccessToken}

			if len(bearerToken) != 2 {
				log.Println("Not valid token.")
				errors.MakeUnathorisedErrorResponse(&w, "Invalid authorization token.")
				return
			}

			claims := &jwtUtils.Claims{}
			token, err := jwt.ParseWithClaims(bearerToken[1], claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					log.Println("Error JWT parsing.")
					return nil, fmt.Errorf("There was an error")
				}
				return []byte(config.SecretKeyAccess), nil
			})

			if !token.Valid {
				ve, ok := err.(*jwt.ValidationError)
				if !ok {
					errors.MakeBadRequestErrorResponse(&w, "Error ValidateionError typecast")
					return
				}

				switch {
				case ve.Errors&jwt.ValidationErrorMalformed != 0:
					log.Println("Not valid token.")
					errors.MakeUnathorisedErrorResponse(&w, "Token is not valid JWT.")
					return
				case ve.Errors&jwt.ValidationErrorExpired != 0:
					if refreshedTokenCreds, err := jwtUtils.RefreshTokens(claims.Username, tokenCreds.RefreshToken, config, userDAO); err != nil {
						log.Println("Token is expired.")
						errors.MakeUnathorisedErrorResponse(&w, err.Error())
						return
					} else {
						log.Println("Refresh tokens")
						jwtUtils.RefreshTokenHeaders(&w, refreshedTokenCreds)
					}
				case ve.Errors&jwt.ValidationErrorNotValidYet != 0:
					log.Println("Error is not valid yet.")
					errors.MakeUnathorisedErrorResponse(&w, "Token is not valid yet.")
					return
				default:
					log.Println("Unhandled error when JWT parsing")
					errors.MakeUnathorisedErrorResponse(&w, "Unhandled error when JWT parsing.")
					return
				}
			}

			userCreds := models.UserCredentials{
				Username:     claims.Username,
				AccessToken:  tokenCreds.AccessToken,
				RefreshToken: tokenCreds.RefreshToken,
			}
			ctxWithCreds := jwtUtils.PassUserCredsInContext(&userCreds, req.Context())
			next.ServeHTTP(w, req.WithContext(ctxWithCreds))
		})
	}
}

func GetTokenCredsFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCreds := &models.TokenCredentials{
			AccessToken:  r.Header.Get("Access"),
			RefreshToken: r.Header.Get("Refresh"),
		}
		ctxWithCreds := jwtUtils.PassTokenCredsInContext(tokenCreds, r.Context())
		next.ServeHTTP(w, r.WithContext(ctxWithCreds))
	})
}

func GetTokenCredsFromBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()

		if err != nil {
			errors.MakeBadRequestErrorResponse(&w, "Expected body keys: [access_token, refresh_token].")
			return
		}

		var tokenCredsFromBody models.TokenCredentials
		err = json.Unmarshal(body, &tokenCredsFromBody)
		if err != nil || tokenCredsFromBody.AccessToken == "" || tokenCredsFromBody.RefreshToken == "" {
			log.Printf("Not valid creds from body: %v %v", err, tokenCredsFromBody)
			errors.MakeBadRequestErrorResponse(&w, "Expected body keys: [access_token, refresh_token].")
			return
		}

		tokenCreds := &models.TokenCredentials{
			AccessToken:  tokenCredsFromBody.AccessToken,
			RefreshToken: tokenCredsFromBody.RefreshToken,
		}
		ctxWithCreds := jwtUtils.PassTokenCredsInContext(tokenCreds, r.Context())
		next.ServeHTTP(w, r.WithContext(ctxWithCreds))
	})
}

