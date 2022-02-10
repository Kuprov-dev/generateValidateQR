package generate

import (
	"context"
	"encoding/json"
	"generateValidateQR/pkg/conf"
	"generateValidateQR/pkg/db"
	"generateValidateQR/pkg/errors"
	"generateValidateQR/pkg/jwt"
	"generateValidateQR/pkg/models"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"image/png"
	"log"
	"net/http"
)

func GenerateHandler(config *conf.Config, userDAO db.UserDAO, ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds models.LoginCredentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			errors.MakeUnathorisedErrorResponse(&w, "Error decoding creds.")
			return
		}
		username := creds.Username
		password := creds.Password
		val:=userDAO.GetByUsername(username)
		if val.Password!=password && val!=nil{
			errors.MakeUnathorisedErrorResponse(&w, "no pass")
			return
		}
		accessToken, accessExpirationTime, err := jwt.CreateAccessToken(username, config.SecretKeyAccess)

		if err != nil {
			errors.MakeInternalServerErrorResponse(&w, "Error create access token.")
		}
		refreshToken, refreshExpirationTime, err := jwt.CreateRefreshToken(username, config.SecretKeyRefresh)
		if err != nil {
			errors.MakeInternalServerErrorResponse(&w, "Error create refresh token.")
		}

		userDAO.UpdateRefreshToken(username, refreshToken)

		http.SetCookie(w, &http.Cookie{
			Name:     "Access",
			Value:    accessToken,
			Path:     "/",
			Expires:  accessExpirationTime,
			HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "Refresh",
			Value:    refreshToken,
			Path:     "/",
			Expires:  refreshExpirationTime,
			HttpOnly: true,
		})

		qrCode, err := qr.Encode(refreshToken, qr.L, qr.Auto)
		if err==nil{
			log.Println(err)

			errors.MakeServiceUnavailableErrorResponse(&w,"qrCode error")
			return
		}
		qrCode, err = barcode.Scale(qrCode, 512, 512)
		if err==nil{
			log.Println(err)

			errors.MakeServiceUnavailableErrorResponse(&w,"qrCode error")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		png.Encode(w, qrCode)
	}
}
func ValidateTokensInBodyHandler(config *conf.Config, userDAO db.UserDAO) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			userCreds, err := jwt.GetUserCredsFromContext(r.Context())
			if err != nil {
				errors.MakeBadRequestErrorResponse(&w, "Couldn't get user creds from context."+err.Error())
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(models.UserDetailResponse{Username: userCreds.Username})
		}
		validateTokenMiddleware := ValidateTokenAndRefreshMiddleware(config, userDAO)
		next := GetTokenCredsFromBody(validateTokenMiddleware(http.HandlerFunc(handler)))
		next.ServeHTTP(w, r)
	}
}
