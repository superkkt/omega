package authenticator

import "github.com/Muzikatoshi/omega/backend"

// TODO: remove this mockup and implement a real authenticator.
type MockAuth struct {
	Username string
	Password string
}

func (r *MockAuth) Auth(userID, password string) (backend.Credential, error) {
	if userID == r.Username && password == r.Password {
		return &mockCredential{auth: true, userUID: 1, userID: userID}, nil
	}

	return &mockCredential{userID: userID}, nil
}

type mockCredential struct {
	auth    bool
	userUID uint64
	userID  string
}

func (r *mockCredential) IsAuthorized() bool {
	return r.auth
}

func (r *mockCredential) UserID() string {
	return r.userID
}

func (r *mockCredential) UserUID() uint64 {
	return r.userUID
}
