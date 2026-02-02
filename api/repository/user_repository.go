package repository

import (
	"easybuy-api/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) CreateOrUpdateUser(email, name, photoURL, clientID string) (*models.User, error) {
	var user models.User

	result := r.db.Where("email = ?", email).First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			user = models.User{
				Email:    email,
				Name:     name,
				PhotoURL: photoURL,
				ClientID: clientID,
			}
			if err := r.db.Create(&user).Error; err != nil {
				return nil, err
			}
			return &user, nil
		}
		return nil, result.Error
	}

	user.Name = name
	user.PhotoURL = photoURL
	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserByID(userID string) (*models.User, error) {
	var user models.User

	if err := r.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User

	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) CreateSession(userID, token string) error {
	session := models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	return r.db.Create(&session).Error
}

func (r *UserRepository) DeleteSession(token string) error {
	return r.db.Where("token = ?", token).Delete(&models.Session{}).Error
}

func (r *UserRepository) ValidateSession(token string) bool {
	var count int64
	r.db.Model(&models.Session{}).Where("token = ?", token).Count(&count)
	return count > 0
}

func (r *UserRepository) GetUserByToken(token string) (*models.User, error) {
	var session models.Session

	if err := r.db.Where("token = ?", token).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	return r.GetUserByID(session.UserID)
}
