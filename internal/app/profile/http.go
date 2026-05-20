package appprofile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	profileusecase "auth_service/internal/usecase/profile"
)

type AuthGrpcClient interface {
	ValidateAccessToken(ctx context.Context, token string) (string, error)
}

func PublicProfileHandler(uc *profileusecase.Usecase, auth AuthGrpcClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/user/me/settings") {
			uid, ok := authUserID(r, auth)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			resp, err := uc.GetSettings(r.Context(), uid)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/user/me/settings") {
			uid, ok := authUserID(r, auth)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var in profileusecase.UpdateProfileSettingsInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			in.UserID = uid
			resp, err := uc.UpdateSettings(r.Context(), in)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/user/") {
			login := strings.TrimPrefix(r.URL.Path, "/v1/user/")
			viewerUserID, _ := authUserID(r, auth)
			resp, err := uc.GetProfileByLogin(r.Context(), login, viewerUserID)
			if err != nil {
				if errors.Is(err, profileusecase.ErrProfileClosed) {
					http.Error(w, "profile is private", http.StatusForbidden)
					return
				}
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	})
}

func authUserID(r *http.Request, auth AuthGrpcClient) (string, bool) {
	p := strings.SplitN(strings.TrimSpace(r.Header.Get("Authorization")), " ", 2)
	if len(p) != 2 {
		return "", false
	}
	uid, err := auth.ValidateAccessToken(r.Context(), p[1])
	return uid, err == nil
}
