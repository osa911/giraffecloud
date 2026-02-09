package repository

import (
	"context"
	"fmt"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/user"
)

// userRepository implements UserRepository interface
type userRepository struct {
	client *ent.Client
}

// NewUserRepository creates a new UserRepository instance
func NewUserRepository(client *ent.Client) UserRepository {
	return &userRepository{
		client: client,
	}
}

func (r *userRepository) Get(ctx context.Context, id uint32) (*ent.User, error) {
	u, err := r.client.User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return nil, err
	}
	return u, nil
}

func (r *userRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*ent.User, error) {
	u, err := r.client.User.Query().
		Where(user.FirebaseUID(firebaseUID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return nil, err
	}
	return u, nil
}

func (r *userRepository) Create(ctx context.Context, user *ent.UserCreate) (*ent.User, error) {
	return user.Save(ctx)
}

func (r *userRepository) Update(ctx context.Context, id uint32, update *ent.UserUpdateOne) (*ent.User, error) {
	u, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return nil, err
	}
	return u, nil
}

func (r *userRepository) Delete(ctx context.Context, id uint32) error {
	err := r.client.User.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return err
	}
	return nil
}

func (r *userRepository) List(ctx context.Context, offset, limit int, search, sortBy, sortOrder string) ([]*ent.User, error) {
	query := r.client.User.Query()

	// Apple filtering
	if search != "" {
		query.Where(user.EmailContainsFold(search))
	}

	// Apply sorting
	if sortBy != "" {
		orderFunc := ent.Asc(sortBy)
		if sortOrder == "desc" {
			orderFunc = ent.Desc(sortBy)
		}

		// Handle specific sort fields
		switch sortBy {
		case "last_login":
			query.Order(orderFunc)
		case "email":
			query.Order(orderFunc)
		default:
			// Default sort by ID desc if invalid sort field
			query.Order(ent.Desc(user.FieldID))
		}
	} else {
		// Default sort
		query.Order(ent.Desc(user.FieldID))
	}

	return query.
		Offset(offset).
		Limit(limit).
		All(ctx)
}

func (r *userRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.client.User.Query().Count(ctx)
	return int64(count), err
}
