package repository

import (
	"buybuddy-api/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

const SessionDuration = 7 * 24 * time.Hour

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

func (r *UserRepository) CreateSession(userID, token string) error {
	session := models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(SessionDuration),
	}

	return r.db.Create(&session).Error
}

func (r *UserRepository) DeleteSession(token string) error {
	return r.db.Where("token = ?", token).Delete(&models.Session{}).Error
}

func (r *UserRepository) SearchByEmail(email string, excludeUserID string, limit int) ([]models.User, error) {
	var users []models.User
	err := r.db.Where("email ILIKE ? AND id != ?", "%"+email+"%", excludeUserID).
		Limit(limit).
		Find(&users).Error
	return users, err
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
