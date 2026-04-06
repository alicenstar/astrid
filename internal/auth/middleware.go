package auth

import (
	"net/http"

	"github.com/redis/go-redis/v9"
)

func NewAuthMiddleware(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("astrid_session")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			session, err := GetSession(rdb, cookie.Value)
			if err != nil || session == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := ContextWithUser(r.Context(), session.UserID, session.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
