package customer

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/mdsami/wpexpress-warper/internal/httpx"
)

const (
	defaultLimit = 50
	minLimit     = 1
	// maxLimit is high enough that ?limit=all returns every customer in one
	// response; it also caps a caller-supplied ?limit=.
	maxLimit = 50000
)

// Handler exposes the customer endpoints over HTTP.
type Handler struct {
	repo *Repository
}

// NewHandler wraps a repository.
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Register attaches the routes to a mux (Go 1.22 method+pattern routing).
// The literal /profiles pattern takes precedence over /{id}.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/customers", h.List)
	mux.HandleFunc("GET /api/customers/{$}", h.List)
	mux.HandleFunc("GET /api/customers/profiles", h.Profiles)
	mux.HandleFunc("GET /api/customers/{id}", h.ByID)
}

// List handles GET /api/customers.
//
//	@Summary		List customers with order totals
//	@Description	Returns WooCommerce customers with their completed/processing order count and total spend, de-duplicated by email address in SQL so that paging never repeats or skips a customer. `total` is the number of unique customers in the database, not just in this page. Use `limit=all` to fetch every customer in one response.
//	@Tags			customers
//	@Produce		json
//	@Param			limit	query		string	false	"Rows per response, or `all` for every customer"	default(50)
//	@Param			offset	query		int		false	"Rows to skip"									minimum(0)	default(0)
//	@Success		200		{object}	CustomerListResponse
//	@Failure		500		{object}	httpx.ErrorBody
//	@Router			/api/customers [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)

	customers, total, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		log.Printf("[List] Error: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "Failed to fetch customer data.", err)
		return
	}

	httpx.JSON(w, http.StatusOK, ListResponse[Customer]{
		Success: true,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Data:    customers,
	})
}

// Profiles handles GET /api/customers/profiles.
//
//	@Summary		List customer profiles
//	@Description	Returns customer profile data without order information, one row per row of wp5u_wc_customer_lookup (so the same email can appear more than once). `total` is the full row count of that table. Use `limit=all` to fetch every row in one response.
//	@Tags			customers
//	@Produce		json
//	@Param			limit	query		string	false	"Rows per response, or `all` for every row"	default(50)
//	@Param			offset	query		int		false	"Rows to skip"								minimum(0)	default(0)
//	@Success		200		{object}	ProfileListResponse
//	@Failure		500		{object}	httpx.ErrorBody
//	@Router			/api/customers/profiles [get]
func (h *Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)

	profiles, total, err := h.repo.Profiles(r.Context(), limit, offset)
	if err != nil {
		log.Printf("[Profiles] Error: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "Failed to fetch customer profiles.", err)
		return
	}

	httpx.JSON(w, http.StatusOK, ListResponse[Profile]{
		Success: true,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Data:    profiles,
	})
}

// ByID handles GET /api/customers/{id}.
//
//	@Summary		Get one customer
//	@Description	Returns a single customer by WooCommerce customer ID, including an order summary and the registration timestamp.
//	@Tags			customers
//	@Produce		json
//	@Param			id	path		int	true	"WooCommerce customer ID"	example(1471)
//	@Success		200	{object}	DetailResponse
//	@Failure		400	{object}	httpx.ErrorBody
//	@Failure		404	{object}	httpx.ErrorBody
//	@Failure		500	{object}	httpx.ErrorBody
//	@Router			/api/customers/{id} [get]
func (h *Handler) ByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Invalid customer ID.", nil)
		return
	}

	customer, err := h.repo.ByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "Customer not found.", nil)
		return
	}
	if err != nil {
		log.Printf("[ByID] Error: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "Failed to fetch customer.", err)
		return
	}

	httpx.JSON(w, http.StatusOK, DetailResponse{Success: true, Data: customer})
}

// pagination clamps ?limit= to [1, maxLimit] (default 50) and ?offset= to
// >= 0. The literal ?limit=all requests every row.
func pagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); strings.EqualFold(raw, "all") {
		limit = maxLimit
	} else if v, err := strconv.Atoi(raw); err == nil && v != 0 {
		limit = v
	}
	limit = min(max(limit, minLimit), maxLimit)

	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		offset = v
	}
	offset = max(offset, 0)

	return limit, offset
}
