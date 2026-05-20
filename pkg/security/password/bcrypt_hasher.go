package password

import "golang.org/x/crypto/bcrypt"

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plain), h.cost)
}

func (h *BcryptHasher) Compare(hash []byte, plain string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(hash, []byte(plain))
	if err == nil {
		return true, nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	return false, err
}
