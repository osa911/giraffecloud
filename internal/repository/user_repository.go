package repository

import (
	"context"

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
	return r.client.User.Get(ctx, id)
}

func (r *userRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*ent.User, error) {
	return r.client.User.Query().
		Where(user.FirebaseUID(firebaseUID)).
		Only(ctx)
}

func (r *userRepository) Create(ctx context.Context, user *ent.UserCreate) (*ent.User, error) {
	return user.Save(ctx)
}

func (r *userRepository) Update(ctx context.Context, id uint32, update *ent.UserUpdateOne) (*ent.User, error) {
	return update.Save(ctx)
}

func (r *userRepository) Delete(ctx context.Context, id uint32) error {
	return r.client.User.DeleteOneID(id).Exec(ctx)
}

func (r *userRepository) List(ctx context.Context, offset, limit int) ([]*ent.User, error) {
	return r.client.User.Query().
		Offset(offset).
		Limit(limit).
		All(ctx)
}

func (r *userRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.client.User.Query().Count(ctx)
	return int64(count), err
}
