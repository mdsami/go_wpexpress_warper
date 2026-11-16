package customer

import (
	"context"
	"database/sql"
)

// Repository reads customer data from the WooCommerce analytics tables
// (wp5u_ prefix) in the imported WordPress database.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps a pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const profilesSQL = `
	SELECT c.customer_id, c.user_id, c.email, c.first_name, c.last_name,
		c.date_registered AS registered_at, c.date_last_active AS last_active_at,
		c.city, c.state, c.postcode, c.country,
		phone.meta_value AS phone,
		address1.meta_value AS address_line1,
		address2.meta_value AS address_line2
	FROM wp5u_wc_customer_lookup c
	LEFT JOIN wp5u_usermeta phone
		ON phone.user_id = c.user_id AND phone.meta_key = 'billing_phone'
	LEFT JOIN wp5u_usermeta address1
		ON address1.user_id = c.user_id AND address1.meta_key = 'billing_address_1'
	LEFT JOIN wp5u_usermeta address2
		ON address2.user_id = c.user_id AND address2.meta_key = 'billing_address_2'
	ORDER BY c.customer_id DESC
	LIMIT ? OFFSET ?`

// Profiles returns customer records without order data, plus the total row
// count of wp5u_wc_customer_lookup.
func (r *Repository) Profiles(ctx context.Context, limit, offset int) ([]Profile, int64, error) {
	rows, err := r.db.QueryContext(ctx, profilesSQL, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	profiles := make([]Profile, 0, limit)
	for rows.Next() {
		var (
			p                              Profile
			userID                         sql.NullInt64
			email, firstName, lastName     sql.NullString
			city, state, postcode, country sql.NullString
			phone, line1, line2            sql.NullString
			registeredAt, lastActiveAt     sql.NullTime
		)

		if err := rows.Scan(
			&p.CustomerID, &userID, &email, &firstName, &lastName,
			&registeredAt, &lastActiveAt,
			&city, &state, &postcode, &country,
			&phone, &line1, &line2,
		); err != nil {
			return nil, 0, err
		}

		p.UserID = num(userID)
		p.Email = str(email)
		p.FirstName = str(firstName)
		p.LastName = str(lastName)
		p.Phone = str(phone)
		p.RegisteredAt = ts(registeredAt)
		p.LastActiveAt = ts(lastActiveAt)
		p.Address = Address{
			Line1:    str(line1),
			Line2:    str(line2),
			City:     str(city),
			State:    str(state),
			Postcode: str(postcode),
			Country:  str(country),
		}

		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) AS total FROM wp5u_wc_customer_lookup`,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return profiles, total, nil
}

// dedupCTE narrows wp5u_wc_customer_lookup to one row per unique email.
// Rows without an email are dropped, and where several rows share an email
// the one with the most orders wins (ties broken by the highest customer_id),
// which is the row the previous in-memory de-duplication would have kept.
const dedupCTE = `
	WITH stats AS (
		SELECT customer_id, COUNT(*) AS orders_count, SUM(total_sales) AS total_spent
		FROM wp5u_wc_order_stats
		WHERE status IN ('wc-completed', 'wc-processing')
		GROUP BY customer_id
	),
	ranked AS (
		SELECT c.customer_id, c.user_id, c.email, c.first_name, c.last_name,
			c.city, c.state, c.postcode, c.country,
			COALESCE(s.orders_count, 0) AS orders_count,
			COALESCE(s.total_spent, 0)  AS total_spent,
			ROW_NUMBER() OVER (
				PARTITION BY LOWER(TRIM(c.email))
				ORDER BY COALESCE(s.orders_count, 0) DESC, c.customer_id DESC
			) AS rn
		FROM wp5u_wc_customer_lookup c
		LEFT JOIN stats s ON s.customer_id = c.customer_id
		WHERE TRIM(COALESCE(c.email, '')) <> ''
	)`

// customersSQL pages over the de-duplicated set. Because de-duplication now
// happens in SQL rather than per page, limit/offset page the whole result
// correctly and no customer is returned twice across pages.
const customersSQL = dedupCTE + `
	SELECT r.customer_id, r.email, r.first_name, r.last_name,
		MAX(NULLIF(phone.meta_value, ''))    AS phone,
		MAX(NULLIF(address1.meta_value, '')) AS address_line1,
		MAX(NULLIF(address2.meta_value, '')) AS address_line2,
		r.city, r.state, r.postcode, r.country,
		r.orders_count, r.total_spent
	FROM ranked r
	LEFT JOIN wp5u_usermeta phone
		ON phone.user_id = r.user_id AND phone.meta_key = 'billing_phone'
	LEFT JOIN wp5u_usermeta address1
		ON address1.user_id = r.user_id AND address1.meta_key = 'billing_address_1'
	LEFT JOIN wp5u_usermeta address2
		ON address2.user_id = r.user_id AND address2.meta_key = 'billing_address_2'
	WHERE r.rn = 1
	GROUP BY r.customer_id, r.email, r.first_name, r.last_name,
		r.city, r.state, r.postcode, r.country, r.orders_count, r.total_spent
	ORDER BY r.orders_count DESC, r.customer_id DESC
	LIMIT ? OFFSET ?`

// customersCountSQL is the number of unique customers in the whole table,
// which is what the `total` field reports.
const customersCountSQL = dedupCTE + `
	SELECT COUNT(*) FROM ranked WHERE rn = 1`

// List returns one page of unique customers plus the total number of unique
// customers in the database.
func (r *Repository) List(ctx context.Context, limit, offset int) ([]Customer, int64, error) {
	rows, err := r.db.QueryContext(ctx, customersSQL, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	customers := make([]Customer, 0, limit)

	for rows.Next() {
		var (
			c                              Customer
			email, firstName, lastName     sql.NullString
			phone, line1, line2            sql.NullString
			city, state, postcode, country sql.NullString
			ordersCount                    sql.NullInt64
			totalSpent                     sql.NullFloat64
		)

		if err := rows.Scan(
			&c.CustomerID, &email, &firstName, &lastName,
			&phone, &line1, &line2,
			&city, &state, &postcode, &country,
			&ordersCount, &totalSpent,
		); err != nil {
			return nil, 0, err
		}

		c.Email = str(email)
		c.FirstName = str(firstName)
		c.LastName = str(lastName)
		c.Phone = str(phone)
		c.Address = Address{
			Line1:    str(line1),
			Line2:    str(line2),
			City:     str(city),
			State:    str(state),
			Postcode: str(postcode),
			Country:  str(country),
		}
		c.OrdersCount = ordersCount.Int64
		c.TotalSpent = totalSpent.Float64

		customers = append(customers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, customersCountSQL).Scan(&total); err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

const customerByIDSQL = `
	SELECT c.customer_id, c.email, c.date_registered AS registered_at,
		c.first_name, c.last_name,
		MAX(NULLIF(phone.meta_value, '')) AS phone,
		MAX(NULLIF(address1.meta_value, '')) AS address_line1,
		MAX(NULLIF(address2.meta_value, '')) AS address_line2,
		c.city, c.state, c.postcode, c.country,
		COUNT(s.order_id) AS orders_count,
		COALESCE(SUM(s.total_sales), 0) AS total_spent
	FROM wp5u_wc_customer_lookup c
	LEFT JOIN wp5u_wc_order_stats s ON s.customer_id = c.customer_id
		AND s.status IN ('wc-completed', 'wc-processing')
	LEFT JOIN wp5u_usermeta phone ON phone.user_id = c.user_id AND phone.meta_key = 'billing_phone'
	LEFT JOIN wp5u_usermeta address1 ON address1.user_id = c.user_id AND address1.meta_key = 'billing_address_1'
	LEFT JOIN wp5u_usermeta address2 ON address2.user_id = c.user_id AND address2.meta_key = 'billing_address_2'
	WHERE c.customer_id = ?
	GROUP BY c.customer_id, c.email, c.date_registered, c.first_name, c.last_name,
		c.city, c.state, c.postcode, c.country`

// ByID returns a single customer. It returns sql.ErrNoRows when the
// customer does not exist.
func (r *Repository) ByID(ctx context.Context, id int64) (CustomerDetail, error) {
	var (
		c                              CustomerDetail
		email, firstName, lastName     sql.NullString
		phone, line1, line2            sql.NullString
		city, state, postcode, country sql.NullString
		registeredAt                   sql.NullTime
		ordersCount                    sql.NullInt64
		totalSpent                     sql.NullFloat64
	)

	err := r.db.QueryRowContext(ctx, customerByIDSQL, id).Scan(
		&c.CustomerID, &email, &registeredAt,
		&firstName, &lastName,
		&phone, &line1, &line2,
		&city, &state, &postcode, &country,
		&ordersCount, &totalSpent,
	)
	if err != nil {
		return CustomerDetail{}, err
	}

	c.Email = str(email)
	c.FirstName = str(firstName)
	c.LastName = str(lastName)
	c.Phone = str(phone)
	c.RegisteredAt = ts(registeredAt)
	c.Address = Address{
		Line1:    str(line1),
		Line2:    str(line2),
		City:     str(city),
		State:    str(state),
		Postcode: str(postcode),
		Country:  str(country),
	}
	c.OrdersCount = ordersCount.Int64
	c.TotalSpent = totalSpent.Float64

	return c, nil
}
