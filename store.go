package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type role string

const (
	roleSeller role = "seller"
	roleAdmin  role = "admin"
)

type vehicleState string

const (
	statePending   vehicleState = "pending"
	statePublished vehicleState = "published"
	stateSold      vehicleState = "sold"
)

const sessionDuration = 8 * time.Hour

var (
	errEmailInUse       = errors.New("email already registered")
	errInvalidLogin     = errors.New("invalid credentials")
	errNotFound         = errors.New("resource not found")
	errForbidden        = errors.New("operation not permitted")
	errInvalidState     = errors.New("invalid resource state")
	errNoAdminRemains   = errors.New("cannot delete the last administrator")
	errCannotDeleteSelf = errors.New("cannot delete the current administrator")
)

type validationError struct {
	message string
}

func (e validationError) Error() string {
	return e.message
}

type user struct {
	ID           string
	Name         string
	Email        string
	Phone        string
	Role         role
	passwordSalt []byte
	passwordHash []byte
	CreatedAt    time.Time
}

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Role      role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type vehicleInput struct {
	Title       string `json:"title"`
	Make        string `json:"make"`
	Model       string `json:"model"`
	Year        int    `json:"year"`
	PriceCents  int64  `json:"price_cents"`
	Description string `json:"description"`
}

type vehicle struct {
	ID          string       `json:"id"`
	SellerID    string       `json:"seller_id"`
	Title       string       `json:"title"`
	Make        string       `json:"make"`
	Model       string       `json:"model"`
	Year        int          `json:"year"`
	PriceCents  int64        `json:"price_cents"`
	Description string       `json:"description"`
	State       vehicleState `json:"state"`
	CreatedAt   time.Time    `json:"created_at"`
}

type publicVehicle struct {
	vehicle
	Seller userResponse `json:"seller"`
}

type session struct {
	UserID    string
	ExpiresAt time.Time
}

type store struct {
	mu       sync.RWMutex
	users    map[string]user
	byEmail  map[string]string
	vehicles map[string]vehicle
	sessions map[string]session
}

func newStore() *store {
	return &store{
		users:    make(map[string]user),
		byEmail:  make(map[string]string),
		vehicles: make(map[string]vehicle),
		sessions: make(map[string]session),
	}
}

func (s *store) createInitialAdmin(email, password string) (userResponse, error) {
	email = normalizeEmail(email)
	if err := validateEmail(email); err != nil {
		return userResponse{}, err
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return userResponse{}, err
	}
	id, err := newID()
	if err != nil {
		return userResponse{}, err
	}

	admin := user{
		ID:           id,
		Name:         "AutoMarket Administrator",
		Email:        email,
		Role:         roleAdmin,
		passwordSalt: salt,
		passwordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.users) != 0 {
		return userResponse{}, errors.New("initial administrator already exists")
	}
	s.users[id] = admin
	s.byEmail[email] = id
	return toUserResponse(admin), nil
}

func (s *store) register(name, email, phone, password string) (userResponse, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)
	phone = strings.TrimSpace(phone)
	if name == "" || len(name) > 120 {
		return userResponse{}, validationError{"name must contain 1 to 120 characters"}
	}
	if err := validateEmail(email); err != nil {
		return userResponse{}, err
	}
	if phone == "" || len(phone) > 32 {
		return userResponse{}, validationError{"phone must contain 1 to 32 characters"}
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return userResponse{}, err
	}
	id, err := newID()
	if err != nil {
		return userResponse{}, err
	}

	account := user{
		ID:           id,
		Name:         name,
		Email:        email,
		Phone:        phone,
		Role:         roleSeller,
		passwordSalt: salt,
		passwordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byEmail[email]; exists {
		return userResponse{}, errEmailInUse
	}
	s.users[id] = account
	s.byEmail[email] = id
	return toUserResponse(account), nil
}

func (s *store) login(email, password string) (userResponse, string, error) {
	email = normalizeEmail(email)
	s.mu.Lock()
	defer s.mu.Unlock()

	id, exists := s.byEmail[email]
	if !exists {
		return userResponse{}, "", errInvalidLogin
	}
	account := s.users[id]
	if !verifyPassword(password, account.passwordSalt, account.passwordHash) {
		return userResponse{}, "", errInvalidLogin
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return userResponse{}, "", err
	}
	token := encodeToken(rawToken)
	s.sessions[token] = session{UserID: id, ExpiresAt: time.Now().UTC().Add(sessionDuration)}
	return toUserResponse(account), token, nil
}

func (s *store) authenticate(token string) (userResponse, bool) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.sessions[token]
	if !exists {
		return userResponse{}, false
	}
	if !now.Before(current.ExpiresAt) {
		delete(s.sessions, token)
		return userResponse{}, false
	}
	account, exists := s.users[current.UserID]
	if !exists {
		delete(s.sessions, token)
		return userResponse{}, false
	}
	return toUserResponse(account), true
}

func (s *store) createVehicle(sellerID string, input vehicleInput) (vehicle, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Make = strings.TrimSpace(input.Make)
	input.Model = strings.TrimSpace(input.Model)
	input.Description = strings.TrimSpace(input.Description)
	if input.Title == "" || len(input.Title) > 160 ||
		input.Make == "" || len(input.Make) > 80 ||
		input.Model == "" || len(input.Model) > 80 ||
		input.Description == "" || len(input.Description) > 4000 {
		return vehicle{}, validationError{"title, make, model, and description are required and must be within their length limits"}
	}
	if input.Year < 1886 || input.Year > time.Now().Year()+1 {
		return vehicle{}, validationError{"year is outside the supported range"}
	}
	if input.PriceCents <= 0 {
		return vehicle{}, validationError{"price_cents must be greater than zero"}
	}
	id, err := newID()
	if err != nil {
		return vehicle{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	account, exists := s.users[sellerID]
	if !exists || account.Role != roleSeller {
		return vehicle{}, errForbidden
	}
	item := vehicle{
		ID:          id,
		SellerID:    sellerID,
		Title:       input.Title,
		Make:        input.Make,
		Model:       input.Model,
		Year:        input.Year,
		PriceCents:  input.PriceCents,
		Description: input.Description,
		State:       statePending,
		CreatedAt:   time.Now().UTC(),
	}
	s.vehicles[id] = item
	return item, nil
}

func (s *store) publicVehicles() []publicVehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]publicVehicle, 0)
	for _, item := range s.vehicles {
		if item.State != statePublished {
			continue
		}
		seller, exists := s.users[item.SellerID]
		if exists {
			result = append(result, publicVehicle{vehicle: item, Seller: toUserResponse(seller)})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *store) publicVehicle(id string) (publicVehicle, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, exists := s.vehicles[id]
	if !exists || item.State != statePublished {
		return publicVehicle{}, false
	}
	seller, exists := s.users[item.SellerID]
	if !exists {
		return publicVehicle{}, false
	}
	return publicVehicle{vehicle: item, Seller: toUserResponse(seller)}, true
}

func (s *store) pendingVehicles() []vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]vehicle, 0)
	for _, item := range s.vehicles {
		if item.State == statePending {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func (s *store) allVehicles() []vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]vehicle, 0, len(s.vehicles))
	for _, item := range s.vehicles {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *store) sellerVehicles(sellerID string) []vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]vehicle, 0)
	for _, item := range s.vehicles {
		if item.SellerID == sellerID {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *store) listUsers() []userResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]userResponse, 0, len(s.users))
	for _, account := range s.users {
		result = append(result, toUserResponse(account))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func (s *store) reportSold(sellerID, vehicleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, exists := s.users[sellerID]
	if !exists || account.Role != roleSeller {
		return errForbidden
	}
	item, exists := s.vehicles[vehicleID]
	if !exists {
		return errNotFound
	}
	if item.SellerID != sellerID {
		return errForbidden
	}
	if item.State == stateSold {
		return errInvalidState
	}
	item.State = stateSold
	s.vehicles[vehicleID] = item
	return nil
}

func (s *store) approveVehicle(adminID, vehicleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	admin, exists := s.users[adminID]
	if !exists || admin.Role != roleAdmin {
		return errForbidden
	}
	item, exists := s.vehicles[vehicleID]
	if !exists {
		return errNotFound
	}
	if item.State != statePending {
		return errInvalidState
	}
	item.State = statePublished
	s.vehicles[vehicleID] = item
	return nil
}

func (s *store) deleteVehicle(adminID, vehicleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	admin, exists := s.users[adminID]
	if !exists || admin.Role != roleAdmin {
		return errForbidden
	}
	if _, exists := s.vehicles[vehicleID]; !exists {
		return errNotFound
	}
	delete(s.vehicles, vehicleID)
	return nil
}

func (s *store) deleteUser(adminID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	admin, exists := s.users[adminID]
	if !exists || admin.Role != roleAdmin {
		return errForbidden
	}
	if adminID == userID {
		return errCannotDeleteSelf
	}
	target, exists := s.users[userID]
	if !exists {
		return errNotFound
	}
	if target.Role == roleAdmin {
		adminCount := 0
		for _, account := range s.users {
			if account.Role == roleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return errNoAdminRemains
		}
	}

	delete(s.users, userID)
	delete(s.byEmail, target.Email)
	for id, item := range s.vehicles {
		if item.SellerID == userID {
			delete(s.vehicles, id)
		}
	}
	for token, current := range s.sessions {
		if current.UserID == userID {
			delete(s.sessions, token)
		}
	}
	return nil
}

func toUserResponse(account user) userResponse {
	return userResponse{
		ID:        account.ID,
		Name:      account.Name,
		Email:     account.Email,
		Phone:     account.Phone,
		Role:      account.Role,
		CreatedAt: account.CreatedAt,
	}
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
