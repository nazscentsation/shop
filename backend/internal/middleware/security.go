package middleware

import "net/http"

// Security adds hardened HTTP response headers.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none';")
		next.ServeHTTP(w, r)
	})
}

// ProtectedFiles gates specific HTML files behind JWT authentication.
// Paths not in the protected list are served freely.
func ProtectedFiles(secret string, comingSoon bool, fileHandler http.Handler) http.Handler {
	// Files that require a valid session (any role)
	userPages := map[string]bool{
		"/shop.html":    true,
		"/account.html": true,
	}
	// Files that require admin role
	adminPages := map[string]bool{
		"/portal.html": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if adminPages[path] {
			claims := AuthClaims(secret, r)
			if claims == nil || claims.Role != "admin" {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		} else if userPages[path] {
			claims := AuthClaims(secret, r)
			if claims == nil {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			// If site is in coming-soon mode, only admins reach the shop
			if comingSoon && claims.Role != "admin" {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}

		fileHandler.ServeHTTP(w, r)
	})
}
