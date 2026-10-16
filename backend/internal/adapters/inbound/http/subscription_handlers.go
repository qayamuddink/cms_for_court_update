package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/livelaw/backend/internal/application/commands"
	"github.com/livelaw/backend/internal/application/ports"
	"github.com/livelaw/backend/internal/application/queries"
	"github.com/livelaw/backend/internal/domain/billing"
	"github.com/livelaw/backend/internal/domain/identity"
)

// PaymentWebhookHandler processes incoming webhooks from payment gateways.
type PaymentWebhookHandler struct {
	useCases *commands.SubscriptionUseCases
}

// NewPaymentWebhookHandler creates a new PaymentWebhookHandler instance.
func NewPaymentWebhookHandler(useCases *commands.SubscriptionUseCases) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{useCases: useCases}
}

// HandleWebhook handles POST /v1/webhooks/payments
func (h *PaymentWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "Failed to read request body", reqID)
		return
	}
	defer r.Body.Close()

	sig := r.Header.Get("X-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Razorpay-Signature")
	}
	if sig == "" {
		sig = r.Header.Get("Stripe-Signature")
	}

	var payload ports.PaymentWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON payload", reqID)
		return
	}

	if err := h.useCases.ProcessPaymentWebhook(r.Context(), bodyBytes, sig, payload); err != nil {
		if errors.Is(err, commands.ErrInvalidWebhookSignature) {
			writeJSONError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Webhook signature verification failed", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReaderSubscriptionHandler handles reader-facing subscription status queries.
type ReaderSubscriptionHandler struct {
	queries *queries.SubscriptionQueries
}

// NewReaderSubscriptionHandler creates a new ReaderSubscriptionHandler instance.
func NewReaderSubscriptionHandler(queries *queries.SubscriptionQueries) *ReaderSubscriptionHandler {
	return &ReaderSubscriptionHandler{queries: queries}
}

// GetMySubscription handles GET /v1/me/subscription
func (h *ReaderSubscriptionHandler) GetMySubscription(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	sub, err := h.queries.GetMySubscription(r.Context(), claims.UserID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get subscription", reqID)
		return
	}

	if sub == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"subscription": nil,
			"tier":         "free",
			"is_entitled":  false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"subscription": sub,
		"tier":         sub.Tier,
		"is_entitled":  sub.IsEntitled(time.Now().UTC()),
	})
}

// CMSSubscriptionHandler handles admin subscription operations in the CMS.
type CMSSubscriptionHandler struct {
	useCases *commands.SubscriptionUseCases
	queries  *queries.SubscriptionQueries
}

// NewCMSSubscriptionHandler creates a new CMSSubscriptionHandler instance.
func NewCMSSubscriptionHandler(
	useCases *commands.SubscriptionUseCases,
	queries *queries.SubscriptionQueries,
) *CMSSubscriptionHandler {
	return &CMSSubscriptionHandler{
		useCases: useCases,
		queries:  queries,
	}
}

// ListSubscriptions handles GET /v1/cms/subscriptions
func (h *CMSSubscriptionHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var statusFilter *billing.SubscriptionStatus
	if sStr := r.URL.Query().Get("status"); sStr != "" {
		st := billing.SubscriptionStatus(sStr)
		statusFilter = &st
	}

	resp, err := h.queries.ListSubscriptions(r.Context(), identity.SystemRole(claims.SystemRole), limit, offset, statusFilter)
	if err != nil {
		if errors.Is(err, ports.ErrUnauthorized) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed querying subscriptions", reqID)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

type grantSubscriptionReq struct {
	UserID       string                   `json:"user_id"`
	Tier         billing.SubscriptionTier `json:"tier"`
	DurationDays int                      `json:"duration_days"`
}

// GrantSubscription handles POST /v1/cms/subscriptions/grant
func (h *CMSSubscriptionHandler) GrantSubscription(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	var req grantSubscriptionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", reqID)
		return
	}

	if req.UserID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_USER", "user_id is required", reqID)
		return
	}
	if req.Tier == "" {
		req.Tier = billing.TierPro
	}
	if req.DurationDays <= 0 {
		req.DurationDays = 30
	}

	sub, err := h.useCases.GrantSubscription(r.Context(), commands.GrantSubscriptionCommand{
		AdminUserID:  claims.UserID,
		TargetUserID: req.UserID,
		Tier:         req.Tier,
		DurationDays: req.DurationDays,
	})
	if err != nil {
		if errors.Is(err, commands.ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Admin role required", reqID)
			return
		}
		if errors.Is(err, commands.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, "USER_NOT_FOUND", "Target user not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

// CancelSubscription handles POST /v1/cms/subscriptions/{id}/cancel
func (h *CMSSubscriptionHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	reqID := GetRequestID(r.Context())
	claims := GetAuthClaims(r.Context())
	if claims == nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", reqID)
		return
	}

	subID := r.PathValue("id")
	if subID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ID", "Subscription ID is required", reqID)
		return
	}

	err := h.useCases.CancelSubscription(r.Context(), claims.UserID, subID)
	if err != nil {
		if errors.Is(err, commands.ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN", "Admin role required", reqID)
			return
		}
		if errors.Is(err, ports.ErrSubscriptionNotFound) {
			writeJSONError(w, http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "Subscription not found", reqID)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), reqID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "id": subID})
}
