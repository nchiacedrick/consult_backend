package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Organisation struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	AboutOrg  string   `json:"about"`
	OwnerID   int64    `json:"owner_id"`
	Purpose   string   `json:"purpose"`
	OrgEmail  string   `json:"org_email"`
	Phone     string   `json:"phone"`
	Website   string   `json:"website"`
	Address   string   `json:"address"`
	Location  string   `json:"location"`
	Founded   string   `json:"founded"`
	Category  string   `json:"category"`
	Logo      string   `json:"logo"`
	Verified  bool     `json:"verified"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Version   int64    `json:"version"`
	Branches  []Branch `json:"branches,omitempty"`
}

type OrganisationStore struct {
	db *sql.DB
}

func (s *OrganisationStore) Create(ctx context.Context, org *Organisation, branch *Branch) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create organisation
	query := `
		INSERT INTO organisations (
			org_name, about_org, owner_id, purpose, org_email, phone, website,
			org_address, org_location, founded_year, category
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err = tx.QueryRowContext(
		ctx,
		query,
		org.Name,
		org.AboutOrg,
		org.OwnerID,
		org.Purpose,
		org.OrgEmail,
		org.Phone,
		org.Website,
		org.Address,
		org.Location,
		org.Founded,
		org.Category,
	).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "organisations_owner_id_key"`:
			return ErrDuplicateOrganisation
		case err.Error() == `pq: duplicate key value violates unique constraint "organisations_org_name_key"`:
			return ErrDuplicateOrganisation
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	// Assign organisation ID to branch explicitly
	branch.OrganisationID = org.ID

	// Create default branch
	branchQuery := `
		INSERT INTO branches (branch_name, about_branch, organisation_id, phone, branch_location)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err = tx.QueryRowContext(ctx, branchQuery,
		branch.Name,
		branch.About,
		branch.OrganisationID,
		branch.Phone,
		branch.BranchLocation,
	).Scan(&branch.ID)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *OrganisationStore) GetAllOrgs(ctx context.Context) (*[]Organisation, error) {

	query := `
		SELECT
			o.id,
			o.org_name,
			o.about_org,
			o.owner_id,
			o.purpose,
			o.org_email,
			o.phone,
			o.website,
			o.org_address,
			o.org_location,
			o.founded_year,
			o.category,
			o.verified,
			o.created_at,
			o.updated_at,
			COALESCE(branches_json, '[]') AS branches
		FROM (
			SELECT
				o.*,
				(
					SELECT json_agg(
						json_build_object(
							'id', b.id,
							'branch_name', b.branch_name,
							'about_branch', b.about_branch,
							'organisation_id', b.organisation_id,
							'phone', b.phone,
							'branch_location', b.branch_location,
							'created_at', b.created_at
						)
					)
					FROM branches b
					WHERE b.organisation_id = o.id
				) AS branches_json
			FROM organisations o
		) o
		ORDER BY o.id;
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orgs := []Organisation{}

	for rows.Next() {

		var org Organisation
		var branchesJSON []byte

		err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.AboutOrg,
			&org.OwnerID,
			&org.Purpose,
			&org.OrgEmail,
			&org.Phone,
			&org.Website,
			&org.Address,
			&org.Location,
			&org.Founded,
			&org.Category,
			&org.Verified,
			&org.CreatedAt,
			&org.UpdatedAt,
			&branchesJSON,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal branches
		if err := json.Unmarshal(branchesJSON, &org.Branches); err != nil {
			return nil, err
		}

		orgs = append(orgs, org)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &orgs, nil
}

type GetOrganisationByIDResult struct {
	Organisation Organisation `json:"organisation"`
}

func (s *OrganisationStore) GetOrganisationByID(ctx context.Context, id int64) (*GetOrganisationByIDResult, error) {

	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT 
			o.id,
			o.org_name,
			o.about_org,
			o.owner_id,
			o.purpose,
			o.org_email,
			o.phone,
			o.website,
			o.org_address,
			o.org_location,
			o.founded_year,
			o.category,
			o.verified,
			o.created_at,
			o.updated_at,
			COALESCE(
				json_agg(
					json_build_object(
						'id', b.id,
						'branch_name', b.branch_name,
						'about_branch', b.about_branch,
						'organisation_id', b.organisation_id,
						'phone', b.phone,
						'branch_location', b.branch_location,
						'created_at', b.created_at,
						'experts', COALESCE(be.experts_json, '[]'::json)
					)
				) FILTER (WHERE b.id IS NOT NULL),
				'[]'::json
			) AS branches
		FROM organisations o
		LEFT JOIN LATERAL (
			SELECT 
				b.id,
				b.branch_name,
				b.about_branch,
				b.organisation_id,
				b.phone,
				b.branch_location,
				b.created_at
			FROM branches b
			WHERE b.organisation_id = o.id
		) b ON TRUE
		LEFT JOIN LATERAL (
			SELECT json_agg(
				json_build_object(
					'id', e.id,
					'user_id', e.user_id,
					'full_name', u.username,
					'email', u.email,
					'specialty', e.expertise
				)
		 ) AS experts_json
			FROM expert_branches eb
			JOIN experts e ON e.id = eb.expert_id
			JOIN users u ON u.id = e.user_id
			WHERE eb.branch_id = b.id
		) be ON TRUE
		WHERE o.id = $1
		GROUP BY o.id;
	`
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var result GetOrganisationByIDResult
	var branchesJSON []byte
  
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&result.Organisation.ID,
		&result.Organisation.Name,
		&result.Organisation.AboutOrg,
		&result.Organisation.OwnerID,
		&result.Organisation.Purpose,
		&result.Organisation.OrgEmail,
		&result.Organisation.Phone,
		&result.Organisation.Website,
		&result.Organisation.Address,
		&result.Organisation.Location,
		&result.Organisation.Founded,
		&result.Organisation.Category,
		&result.Organisation.Verified,
		&result.Organisation.CreatedAt,
		&result.Organisation.UpdatedAt,
		&branchesJSON,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	// Unmarshal branches
	if err := json.Unmarshal(branchesJSON, &result.Organisation.Branches); err != nil {
		return nil, err
	}

	return &result, nil		
}

func (s *OrganisationStore) Update(ctx context.Context, org *Organisation) error {

	query := `
		UPDATE organisations
		SET org_name = $1, about_org = $2, version = version + 1
		WHERE id = $3
		RETURNING version
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	return s.db.QueryRowContext(ctx, query, org.Name, org.AboutOrg, org.ID).Scan(&org.Version)
}

func (s *OrganisationStore) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
	 DELETE FROM organisations WHERE id = $1 
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, id)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (s *OrganisationStore) Verify(ctx context.Context, org Organisation) error {
	query := `
		UPDATE organisations
		SET verified = true, version = version + 1
		WHERE id = $1
		RETURNING version 
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	if err := s.db.QueryRowContext(ctx, query, org.ID).Scan(&org.Version); err != nil {
		return err
	}

	return nil
}

func (s *OrganisationStore) GetOwnerID(ctx context.Context, id int64) (int64, error) {
	if id < 1 {
		return 0, ErrRecordNotFound
	}

	query := `
		SELECT owner_id FROM organisations WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var ownerID int64
	err := s.db.QueryRowContext(ctx, query, id).Scan(&ownerID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return 0, ErrRecordNotFound
		default:
			return 0, err
		}
	}

	return ownerID, nil
}

func (s *OrganisationStore) GetAllForUser(ctx context.Context, userID int64) (*[]Organisation, error) {
	query := `
        SELECT id, org_name, about_org, owner_id, created_at, updated_at, version
        FROM organisations
        WHERE owner_id = $1
        ORDER BY created_at DESC
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var organisations []Organisation
	for rows.Next() {
		var org Organisation
		err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.AboutOrg,
			&org.OwnerID,
			&org.CreatedAt,
			&org.UpdatedAt,
			&org.Version,
		)
		if err != nil {
			return nil, err
		}
		organisations = append(organisations, org)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &organisations, nil
}

func (s *OrganisationStore) GetAllBranchesForOrganisation(ctx context.Context, orgID int64) (*[]Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches
		WHERE organisation_id = $1	
		ORDER BY created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var branch Branch
		err := rows.Scan(
			&branch.ID,
			&branch.Name,
			&branch.About,
			&branch.OrganisationID,
			&branch.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &branches, nil
}

func (s *OrganisationStore) IsVerified(ctx context.Context, orgID int64) (bool, error) {
	query := `
        SELECT verified 
        FROM organisations 
        WHERE id = $1
    `

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var isVerified bool
	err := s.db.QueryRowContext(ctx, query, orgID).Scan(&isVerified)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return false, ErrRecordNotFound
		default:
			return false, err
		}
	}

	return isVerified, nil
}

func (s *OrganisationStore) IsOwner(ctx context.Context, userID, orgID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM organisations 
			WHERE id = $1 AND owner_id = $2
		)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var exists bool
	err := s.db.QueryRowContext(ctx, query, orgID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
