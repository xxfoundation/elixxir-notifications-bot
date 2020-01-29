package storage

import "testing"

// NOT A UNIT TEST
// Test functionality of DB functions on a local postgres instance
func TestNewDatabase(t *testing.T) {
	s := NewDatabase("postgres", "", "test", "0.0.0.0:5432")
	u1 := &User{
		Id:    "testid",
		Token: "testtoken",
	}
	err := s.UpsertUser(u1)
	if err != nil {
		t.Errorf("Failed to insert user: %+v", err)
	}

	u2, err := s.GetUser("testid")
	if err != nil {
		t.Errorf("Failed to get user: %+v", err)
	}

	if u2.Token == "" || u2.Token != u1.Token {
		t.Errorf("Token retrieved was not same as one stored")
	}

	u3 := &User{
		Id:    "testid",
		Token: "testtoken2",
	}
	err = s.UpsertUser(u3)
	if err != nil {
		t.Errorf("Failed to upsert user: %+v", err)
	}

	u4, err := s.GetUser("testid")
	if err != nil {
		t.Errorf("Failed to get upserted user: %+v", err)
	}
	if u4.Token != u3.Token {
		t.Errorf("Did not properly update token for existing user")
	}

	err = s.DeleteUser("testid")
	if err != nil {
		t.Errorf("Failed to delete user: %+v", err)
	}

	_, err = s.GetUser("testid")
	if err == nil {
		t.Errorf("GetUser should not have found anything after delete")
	}
}
