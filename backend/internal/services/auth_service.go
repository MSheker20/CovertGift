package services

import (
	"errors"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Standart hata tanımları (Sentinel Errors)
var (
	ErrEmailAlreadyExists = errors.New("bu e-posta adresi zaten kullanımda")
	ErrInvalidCredentials = errors.New("geçersiz e-posta veya şifre")
	ErrUserNotFound       = errors.New("kullanıcı bulunamadı")
)

// JWTClaims token içinde taşınacak özel verileri tanımlar
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// AuthService kimlik doğrulama iş mantığı sözleşmesidir
type AuthService interface {
	Register(name, email, password string) (*models.User, error)
	Login(email, password string) (*models.User, string, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
	GenerateToken(user *models.User) (string, error)
}

type authService struct {
	userRepo  repository.UserRepository
	jwtSecret string
}

// NewAuthService yeni bir AuthService örneği oluşturur
func NewAuthService(userRepo repository.UserRepository, jwtSecret string) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// Register yeni kullanıcı kaydı mantığını işletir
func (s *authService) Register(name, email, password string) (*models.User, error) {
	// 1. Bu e-posta ile kayıtlı kullanıcı var mı kontrol et
	existing, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	// 2. Şifreyi bcrypt ile hash'le
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	// 3. Veritabanına yaz
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login giriş bilgilerini doğrular ve geçerliyse token üretir
func (s *authService) Login(email, password string) (*models.User, string, error) {
	// 1. Kullanıcıyı bul
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, "", err
	}
	// Kullanıcı yoksa aynı jenerik hatayı dön (User Enumeration koruması)
	if user == nil {
		return nil, "", ErrInvalidCredentials
	}

	// 2. Şifre eşleşiyor mu kontrol et
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// 3. 24 saat geçerli JWT token üret
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// GetUserByID profil sorgusu için kullanıcıyı getirir
func (s *authService) GetUserByID(id uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GenerateToken HS256 algoritmasıyla imzalanmış JWT üretir
func (s *authService) GenerateToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
