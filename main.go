package main

import "bufio"
import "fmt"
import "log"
import "net"
import "strconv"
import "strings"
import "time"
import c "github.com/fimmtiu/gofingerd/config"
import r "github.com/fimmtiu/gofingerd/request"
import u "github.com/fimmtiu/gofingerd/users"

var conf c.Config

func main() {
	conf = c.ReadConfig()

	// FIXME: It would be nice to provide an option to daemonize the process here.

	// Open the listening socket
	listener, err := net.Listen("tcp", ":" + strconv.Itoa(conf.Port))
	if listener == nil {
		log.Fatalf("ERROR: Can't listen on port %d: %s", conf.Port, err.Error())
	}
	defer listener.Close()
	log.Printf("Listening on port %d.", conf.Port)

	// For every incoming connection, spin up a goroutine to handle the request.
	for {
		conn, err := listener.Accept()
		if err == nil {
			go handleConnection(conn)
		} else {
			log.Fatalf("ERROR: Couldn't accept connection: %s", err.Error())
		}
	}
}

// Read user input and send a response on a single client connection.
func handleConnection(conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(conf.NetworkTimeout))
	request, err := readRequest(conn)
	if err != nil {
		log.Printf("ERROR: Can't read from %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	if len(request.Hosts) > 0 {
		// FIXME: We should allow an option to permit forwarding, disabled by default.
		log.Printf("Refused forwarding request from %s.", conn.RemoteAddr().String())
		ConnWritef(conn, "No forwarding for you!")
		return
	}

	if len(request.User) == 0 {
		if conf.AllowUserListing {
			RespondWithUserList(conn)
		} else {
			log.Printf("Refused user listing request from %s.", conn.RemoteAddr().String())
			ConnWritef(conn, "No results; user listing disabled by administrator.")
		}
	} else if conf.AllowApproximateSearch {
		RespondWithApproximateSearch(conn, request)
	} else {
		RespondWithExactSearch(conn, request)
	}
}

// Parse a line of user input into a Request object.
func readRequest(conn net.Conn) (r.Request, error) {
	input, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return r.ParseRequest(""), err
	}
	input = strings.TrimSuffix(input, "\r\n")
	input = strings.TrimSuffix(input, "\n")
	log.Printf("Received request from %s: \"%s\"", conn.RemoteAddr().String(), input)
	return r.ParseRequest(input), nil
}

// Show a list of all users on the system.
func RespondWithUserList(conn net.Conn) {
	users, err := u.ListAllUsers()
	if err != nil {
		log.Printf("ERROR: Can't enumerate users for %s: %s.", conn.RemoteAddr().String(), err.Error())
		ConnWritef(conn, "An error occurred. Please try again later.")
		return
	}

	ConnWritef(conn, "User Name         Full Name")
	for _, user := range users {
		ConnWritef(conn, "%-16s  %s", user.LoginName, user.FullName)
	}
}

// Show all users whose name contains the client's input.
func RespondWithApproximateSearch(conn net.Conn, request r.Request) {
	users, err := u.SearchUsers(request.User)
	if err != nil {
		ConnWritef(conn, "An error occurred. Please try again later.")
		log.Printf("ERROR: Can't enumerate users for %s: %s", conn.RemoteAddr().String(), err.Error())
		return
	}

	if len(users) == 0 {
		log.Printf("Found no users matching query \"%s\" from %s.", request.User, conn.RemoteAddr().String())
		ConnWritef(conn, "Found no users matching your query.")
	} else {
		log.Printf("Found %d users matching query \"%s\" from %s.", len(users), request.User, conn.RemoteAddr().String())
		for _, user := range users {
			RespondWithUser(conn, user)
		}
	}
}

// Show the user whose exact name was provided by the client.
func RespondWithExactSearch(conn net.Conn, request r.Request) {
	user, err := u.FindUser(request.User)
	if err != nil {
		log.Printf("ERROR: Can't look up user \"%s\" for %s: %s", request.User, conn.RemoteAddr().String(), err.Error())
		ConnWritef(conn, "An error occurred. Please try again later.")
		return
	}

	if len(user.LoginName) > 0 {
		RespondWithUser(conn, user)
	} else {
		log.Printf("Found no user matching query \"%s\" for %s.", request.User, conn.RemoteAddr().String())
		ConnWritef(conn, "Found no users matching your query.")
	}
}

// Show a detailed description of an individual user.
func RespondWithUser(conn net.Conn, user u.User) {
	ConnWritef(conn, "Login name: %-30s Home directory: %s", user.LoginName, user.HomeDir)
	ConnWritef(conn, "Full name: %-31s Shell: %s", user.FullName, user.Shell)
	conn.Write(user.GetPlanFile())
	conn.Write([]byte("\r\n"))
}

// A little helper function which sends a single line to the client.
func ConnWritef(conn net.Conn, format string, a ...interface{}) (int, error) {
	line := fmt.Sprintf(format, a...)
	line += "\r\n"
	return conn.Write([]byte(line))
}
