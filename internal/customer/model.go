// Package customer contains the data model, SQL access and HTTP handlers
// for the customer endpoints.
package customer

import (
	"database/sql"
	"time"
)

// Time serialises as an ISO-8601 UTC string, matching the output of
// JSON.stringify(new Date(...)) in the Node version.
type Time time.Time

// MarshalJSON renders the timestamp as "2006-01-02T15:04:05.000Z".
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z") + `"`), nil
}

// Address is the billing address block embedded in every customer record.
type Address struct {
	Line1    *string `json:"line1" extensions:"x-nullable" example:"badda link road"`
	Line2    *string `json:"line2" extensions:"x-nullable" example:"Ja-128"`
	City     *string `json:"city" extensions:"x-nullable" example:"Dhaka"`
	State    *string `json:"state" extensions:"x-nullable" example:"BD-13"`
	Postcode *string `json:"postcode" extensions:"x-nullable" example:"1212"`
	Country  *string `json:"country" extensions:"x-nullable" example:"BD"`
}

// Customer is the payload of GET /api/customers.
type Customer struct {
	CustomerID  int64   `json:"customer_id" example:"1471"`
	Email       *string `json:"email" extensions:"x-nullable" example:"customer@example.com"`
	FirstName   *string `json:"first_name" extensions:"x-nullable" example:"Mariyam"`
	LastName    *string `json:"last_name" extensions:"x-nullable" example:"Akther"`
	Phone       *string `json:"phone" extensions:"x-nullable" example:"01795088307"`
	Address     Address `json:"address"`
	OrdersCount int64   `json:"orders_count" example:"4"`
	TotalSpent  float64 `json:"total_spent" example:"7490"`
}

// CustomerDetail is the payload of GET /api/customers/:id. It carries the
// registration timestamp in addition to the list fields.
type CustomerDetail struct {
	CustomerID   int64   `json:"customer_id" example:"1471"`
	Email        *string `json:"email" extensions:"x-nullable" example:"customer@example.com"`
	FirstName    *string `json:"first_name" extensions:"x-nullable" example:"Mariyam"`
	LastName     *string `json:"last_name" extensions:"x-nullable" example:"Akther"`
	Phone        *string `json:"phone" extensions:"x-nullable" example:"01795088307"`
	RegisteredAt *Time   `json:"registered_at" swaggertype:"string" format:"date-time" extensions:"x-nullable" example:"2021-04-07T19:47:51.000Z"`
	Address      Address `json:"address"`
	OrdersCount  int64   `json:"orders_count" example:"4"`
	TotalSpent   float64 `json:"total_spent" example:"7490"`
}

// Profile is the payload of GET /api/customers/profiles.
type Profile struct {
	CustomerID   int64   `json:"customer_id" example:"12811"`
	UserID       *int64  `json:"user_id" extensions:"x-nullable" example:"19897"`
	Email        *string `json:"email" extensions:"x-nullable" example:"customer@example.com"`
	FirstName    *string `json:"first_name" extensions:"x-nullable" example:"farhana"`
	LastName     *string `json:"last_name" extensions:"x-nullable" example:"faria"`
	Phone        *string `json:"phone" extensions:"x-nullable" example:"01795088307"`
	RegisteredAt *Time   `json:"registered_at" swaggertype:"string" format:"date-time" extensions:"x-nullable" example:"2022-02-27T14:10:53.000Z"`
	LastActiveAt *Time   `json:"last_active_at" swaggertype:"string" format:"date-time" extensions:"x-nullable" example:"2022-05-11T10:00:00.000Z"`
	Address      Address `json:"address"`
}

// DetailResponse is the envelope returned by GET /api/customers/:id.
type DetailResponse struct {
	Success bool           `json:"success" example:"true"`
	Data    CustomerDetail `json:"data"`
}

// ListResponse is the envelope returned by both list endpoints.
type ListResponse[T any] struct {
	Success bool  `json:"success" example:"true"`
	Total   int64 `json:"total" example:"12578"`
	Limit   int   `json:"limit" example:"50"`
	Offset  int   `json:"offset" example:"0"`
	Data    []T   `json:"data"`
}

// CustomerListResponse is ListResponse[Customer]; declared concretely so the
// generated spec carries a named schema (swag does not expand generics).
type CustomerListResponse struct {
	Success bool       `json:"success" example:"true"`
	Total   int64      `json:"total" example:"25"`
	Limit   int        `json:"limit" example:"50"`
	Offset  int        `json:"offset" example:"0"`
	Data    []Customer `json:"data"`
}

// ProfileListResponse is ListResponse[Profile] as a named schema.
type ProfileListResponse struct {
	Success bool      `json:"success" example:"true"`
	Total   int64     `json:"total" example:"12578"`
	Limit   int       `json:"limit" example:"50"`
	Offset  int       `json:"offset" example:"0"`
	Data    []Profile `json:"data"`
}

// str converts a nullable column to *string, mapping "" to nil so that
// empty values serialise as null exactly like `row.x || null` in JS.
func str(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	v := ns.String
	return &v
}

func num(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func ts(nt sql.NullTime) *Time {
	if !nt.Valid || nt.Time.IsZero() {
		return nil
	}
	v := Time(nt.Time)
	return &v
}
