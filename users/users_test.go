package users

import os_user "os/user"
import "testing"

func TestListAllUsers(t *testing.T) {
	users, err := ListAllUsers()
	if err != nil || len(users) < 2 || !containsCurrentUser(users) {
		t.Fail()
	}
}

func TestFindUser_UserFound(t *testing.T) {
	user, err := FindUser(currentUserName())
	if err != nil || len(user.LoginName) == 0 {
		t.Fail()
	}
}

func TestFindUser_UserNotFound(t *testing.T) {
	user, err := FindUser("Wanko McCheesenose")
	if err != nil || len(user.LoginName) > 0 {
		t.Fail()
	}
}

func TestSearchUser_UserFound(t *testing.T) {
	users, err := SearchUsers(currentUserName()[0:3])
	if err != nil || len(users) == 0 || !containsCurrentUser(users) {
		t.Fail()
	}
}

func TestSearchUser_UserNotFound(t *testing.T) {
	users, err := SearchUsers("Honk LeBonk")
	if err != nil || len(users) > 0 {
		t.Fail()
	}
}

func containsCurrentUser(users []User) bool {
	currentUser := currentUserName()
	for _, user := range users {
		if user.LoginName == currentUser {
			return true
		}
	}
	return false
}

func currentUserName() string {
	name, err := os_user.Current()
	if err != nil {
		panic("Seriously, WTF?")
	}
	return name.Username
}
