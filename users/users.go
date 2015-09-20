package users

// #include <stdio.h>
// #include <errno.h>
// #include <sys/types.h>
// #include <pwd.h>
// #include <uuid/uuid.h>
import "C"
import "io/ioutil"
import "log"
import "os"
import "strings"

// All of this runs in a separate goroutine service so that getpwent &
// friends don't get called from more than one goroutine at a time. Because
// they're not re-entrant, requests which involve searching the user
// database could return inconsistent results if they're called from
// multiple goroutines at once.
//
// (The curious reader will no doubt wonder "Wouldn't it be vastly simpler
// to just fork a subprocess for each connection, thus entirely avoiding
// the issues of re-entrancy and resource contention?" Well, yes. Yes it
// would. But the point here was to teach myself Go idioms, so...)

const (
	ListAll = 0
	Search = 1
	Find = 2
)

type User struct {
	LoginName string
	FullName string
	HomeDir string
	Shell string
}

// The request and response parameters are all in one struct so that we can
// just modify the struct we're given and send it back, thus avoiding
// allocating new objects that get passed between goroutines. (It's bad
// enough that we have to allocate the response channels on every request
// in the first place.)
type UserCommand struct {
	Type int
	ResponseChannel chan UserCommand
	Name string
	Users []User
	Err   error
}

func StartService(channel chan UserCommand) {
	for {
		cmd := <- channel
		switch cmd.Type {
		case ListAll:
			cmd.Users, cmd.Err = ListAllUsers()
		case Search:
			cmd.Users, cmd.Err = SearchUsers(cmd.Name)
		case Find:
			users, err := FindUser(cmd.Name)
			cmd.Users, cmd.Err = users, err
		}
		cmd.ResponseChannel <- cmd
	}
}

// Returns a list of all users on the system.
func ListAllUsers() ([]User, error) {
	users := []User{}
	for {
		pwent, err := C.getpwent()
		if pwent == nil {
			if err != nil {
				return nil, err
			} else {
				break
			}
		}
		users = append(users, pwentToUser(pwent))
	}
	C.endpwent()
	return users, nil
}

// Returns the User struct for a particular login name.
func FindUser(name string) ([]User, error) {
	pwent, err := C.getpwnam(C.CString(name))
	if pwent == nil {
		return []User{}, err
	}
	return []User{pwentToUser(pwent)}, nil
}

// Return a list of those users on the system whose login name or full name
// contains the given string.
func SearchUsers(name string) ([]User, error) {
	name = strings.ToLower(name)
	users, err := ListAllUsers()
	if err != nil {
		return nil, err
	}

	filtered := []User{}
	for _, user := range users {
		if strings.Contains(user.LoginName, name) || strings.Contains(user.FullName, name) {
			filtered = append(filtered, user)
		}
	}
	return filtered, nil
}

// .plan files are traditionally quite short, so for the moment I won't
// worry about memory usage from reading abnormally large ones. Something
// to consider for later, though.
func (u *User) GetPlanFile() []byte {
	plan, err := ioutil.ReadFile(u.HomeDir + "/.plan")
	if err == nil {
		header := []byte("Plan:\r\n")
		return append(header, plan...)
	} else if !os.IsNotExist(err) {
		log.Printf("Couldn't read .plan file for %s: %s", u.LoginName, err.Error())
	}
	return []byte("No plan.")
}

// Converts a C "struct passwd" to a User struct.
func pwentToUser(pwent *C.struct_passwd) User {
	return User{
		C.GoString(pwent.pw_name),
		C.GoString(pwent.pw_gecos),
		C.GoString(pwent.pw_dir),
		C.GoString(pwent.pw_shell),
	}
}
