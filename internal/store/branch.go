package store

import (
	"context"
	"database/sql"
	"errors"
)

type Branch struct {
	ID             int64    `json:"id"`
	Name           string   `json:"branch_name"`
	OrganisationID int64    `json:"organisation_id"`
	About          string   `json:"about_branch"`
	Phone          string   `json:"phone"`
	BranchLocation string   `json:"branch_location"`
	Experts        []Expert `json:"experts,omitempty"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}  

type BranchStore struct {
	db *sql.DB
}

func (s *BranchStore) Create(ctx context.Context, branch *Branch) error {
	query := `
	INSERT INTO branches (branch_name, about_branch, organisation_id) 
	VALUES($1, $2, $3) 
	RETURNING id, created_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, branch.Name, branch.About, branch.OrganisationID).Scan(&branch.ID, &branch.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *BranchStore) GetByID(ctx context.Context, id int64) (*Branch, error) {
	query := `
		SELECT
			b.id,
			b.branch_name,
			b.about_branch,
			b.organisation_id,
			b.created_at,
			b.updated_at,
			COALESCE(
				json_agg(
					json_build_object(
						'id', u.id,
						'name', u.username,
						'email', u.email,
						'phone', u.phone,
						'created_at', u.created_at,
						'updated_at', u.updated_at
					)
				) FILTER (WHERE u.id IS NOT NULL),
				'[]'
			) AS experts
		FROM branches b
		LEFT JOIN expert_branches eb ON eb.branch_id = b.id
		LEFT JOIN experts e ON e.id = eb.expert_id
		LEFT JOIN users u ON u.id = e.user_id
		WHERE b.id = $1
		GROUP BY b.id, b.branch_name, b.about_branch, b.organisation_id, b.created_at, b.updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var branch Branch
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}
	return &branch, nil
}

func (s *BranchStore) Update(ctx context.Context, branch *Branch) error {
	query := `
		UPDATE branches 
		SET branch_name = $1, about_branch = $2, organisation_id = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query,
		branch.Name, branch.About, branch.OrganisationID, branch.ID,
	).Scan(&branch.ID, &branch.UpdatedAt)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrNotFound
		default:
			return err
		}
	}

	return nil
}

func (s *BranchStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM branches WHERE id = $1`

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
		return ErrNotFound
	}

	return nil
}

// Get all branches belonging to an organisation
func (s *BranchStore) GetAllOrganisationBranches(ctx context.Context, organisationID int64) (*[]Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches
		WHERE organisation_id = $1
		ORDER BY created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, organisationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var branch Branch
		err := rows.Scan(
			&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
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

// Get all branches belonging to an expert
func (s *BranchStore) GetAllExpertBranches(ctx context.Context, expertID int64) (*[]Branch, error) {
	query := `
		SELECT b.id, b.branch_name, b.about_branch, b.organisation_id, b.created_at, b.updated_at
		FROM branches b
		INNER JOIN expert_branches eb ON eb.branch_id = b.id 
		WHERE eb.expert_id = $1
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, expertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var branch Branch
		err := rows.Scan(
			&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
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

// Get branch by organisation ID
func (s *BranchStore) GetBranchByOrgID(ctx context.Context, organisationID int64) (*[]Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches
		WHERE organisation_id = $1
		ORDER BY created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, organisationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var branch Branch
		err := rows.Scan(
			&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
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

// Get branch by expert ID
func (s *BranchStore) GetBranchByExpertID(ctx context.Context, expertID int64) (*Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches b		
		INNER JOIN expert_branches eb ON eb.branch_id = b.id
		WHERE eb.expert_id = $1
		ORDER BY b.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var branch Branch
	err := s.db.QueryRowContext(ctx, query, expertID).Scan(
		&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &branch, nil
}

// Get all branches
func (s *BranchStore) GetAllBranches(ctx context.Context) (*[]Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches 
		ORDER BY created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var branch Branch
		err := rows.Scan(
			&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
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

// Get branch by ID
func (s *BranchStore) GetBranchByID(ctx context.Context, id int64) (*Branch, error) {
	query := `
		SELECT id, branch_name, about_branch, organisation_id, created_at, updated_at
		FROM branches
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var branch Branch
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&branch.ID, &branch.Name, &branch.About, &branch.OrganisationID, &branch.CreatedAt, &branch.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return &branch, nil
}

// Get all experts for a branch
func (s *BranchStore) GetAllExpertsForBranch(ctx context.Context, branchID int64) (*[]Expert, error) {
	query := `
		SELECT u.id, u.username, u.email, u.phone, u.created_at, u.updated_at
		FROM users u
		INNER JOIN experts e ON e.user_id = u.id
		INNER JOIN expert_branches eb ON eb.expert_id = e.id
		WHERE eb.branch_id = $1
		ORDER BY u.created_at DESC
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var experts []Expert
	for rows.Next() {
		var expert Expert
		err := rows.Scan(
			&expert.ID, &expert.Name, &expert.Email, &expert.Phone, &expert.CreatedAt, &expert.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		experts = append(experts, expert)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &experts, nil
}

// IsOwner checks if a user is the owner of a branch by checking if they are the owner of the associated organisation
func (s *BranchStore) IsOwner(ctx context.Context, userID, branchID int64) (bool, error) {
	query := `
		SELECT o.owner_id
		FROM branches b
		INNER JOIN organisations o ON o.id = b.organisation_id
		WHERE b.id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var ownerID int64
	err := s.db.QueryRowContext(ctx, query, branchID).Scan(&ownerID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return false, ErrNotFound
		default:
			return false, err
		}
	}

	return ownerID == userID, nil
}

func (s *BranchStore) RemoveExpertFromBranch(ctx context.Context, branchID, expertID int64) error {
	query := `
		DELETE FROM expert_branches WHERE branch_id = $1 AND expert_id = $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, branchID, expertID)
	if err != nil {
		return err
	}

	return nil
}

// AddExpertToBranch adds an expert to a branch
func (s *BranchStore) AddExpertToBranch(ctx context.Context, branchID, expertID int64) error {
	query := `
		INSERT INTO expert_branches (branch_id, expert_id)
		VALUES ($1, $2)
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, branchID, expertID)
	if err != nil {
		return err
	}

	return nil
}