package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"obzornik-pbn-generator/internal/auth"
)

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Если Origin пуст (curl, Prometheus и т.п.), пропускаем без запрета.
		if origin == "" {
			if len(s.cfg.AllowedOrigins) > 0 {
				origin = s.cfg.AllowedOrigins[0]
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			if len(s.cfg.AllowedOrigins) > 0 && !isOriginAllowed(origin, s.cfg.AllowedOrigins) {
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const tokenContextKey contextKey = "token"
const userEmailContextKey contextKey = "userEmail"
const sessionContextKey contextKey = "session"
const currentUserContextKey contextKey = "currentUser"

var (
	errForbidden       = errors.New("forbidden")
	errUnauthorized    = errors.New("unauthorized")
	errPendingApproval = errors.New("account pending approval")
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getAccessFromCookie(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sess, err := s.svc.ValidateToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := s.svc.GetUser(r.Context(), sess.Email)
		if err != nil {
			s.logger.Warnw("failed to load user for session", "email", sess.Email, "err", err)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		ctx = context.WithValue(ctx, userEmailContextKey, sess.Email)
		ctx = context.WithValue(ctx, sessionContextKey, sess)
		ctx = context.WithValue(ctx, currentUserContextKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenFromContext(ctx context.Context) string {
	val, _ := ctx.Value(tokenContextKey).(string)
	return val
}

func userEmailFromContext(ctx context.Context) string {
	val, _ := ctx.Value(userEmailContextKey).(string)
	return val
}

func sessionFromContext(ctx context.Context) auth.Session {
	val, _ := ctx.Value(sessionContextKey).(auth.Session)
	return val
}

func currentUserFromContext(ctx context.Context) (auth.User, bool) {
	val, ok := ctx.Value(currentUserContextKey).(auth.User)
	return val, ok
}

func requireApprovedUser(u auth.User) error {
	if strings.EqualFold(u.Role, "admin") {
		return nil
	}
	if !u.IsApproved {
		return errPendingApproval
	}
	return nil
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUserFromContext(r.Context())
		if !ok || !strings.EqualFold(user.Role, "admin") {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getAccessFromCookie(r *http.Request) string {
	c, err := r.Cookie("access_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func getRefreshFromCookie(r *http.Request) string {
	c, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	return c.Value
}

func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

type ctxKey string

const reqIDKey ctxKey = "reqID"

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := context.WithValue(r.Context(), reqIDKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	val, _ := ctx.Value(reqIDKey).(string)
	return val
}

type statusWriter struct {
	http.ResponseWriter
	status     int
	errCode    string
	errMessage string
	errDetails string
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) SetErrorMeta(code, message, details string) {
	if trimmedCode := strings.TrimSpace(code); trimmedCode != "" {
		w.errCode = trimmedCode
	}
	if trimmedMessage := strings.TrimSpace(message); trimmedMessage != "" {
		w.errMessage = trimmedMessage
	}
	if trimmedDetails := strings.TrimSpace(details); trimmedDetails != "" {
		w.errDetails = trimmedDetails
	}
	if parent, ok := w.ResponseWriter.(errorMetaWriter); ok {
		parent.SetErrorMeta(code, message, details)
	}
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		labels := prometheus.Labels{
			"method": r.Method,
			"path":   r.URL.Path,
			"status": strconv.Itoa(sw.status),
		}
		s.reqDuration.With(labels).Observe(time.Since(start).Seconds())
		s.reqCounter.With(labels).Inc()
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start)
		rid := requestIDFromContext(r.Context())
		baseFields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Seconds() * 1000,
			"ip", clientIP(r),
			"request_id", rid,
		}
		if sw.status >= http.StatusBadRequest {
			errorCode := strings.TrimSpace(sw.errCode)
			if errorCode == "" {
				errorCode = defaultErrorCode(sw.status)
			}
			errorMessage := strings.TrimSpace(sw.errMessage)
			if errorMessage == "" {
				errorMessage = http.StatusText(sw.status)
			}
			errorKind := "client"
			if sw.status >= http.StatusInternalServerError {
				errorKind = "server"
			}
			baseFields = append(baseFields,
				"error_code", errorCode,
				"error_message", errorMessage,
				"error_kind", errorKind,
				"error_details", sw.errDetails,
			)
			if sw.status >= http.StatusInternalServerError {
				s.logger.Errorw("request_error", baseFields...)
				return
			}
			s.logger.Warnw("request_error", baseFields...)
			return
		}
		s.logger.Infow("request", baseFields...)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
